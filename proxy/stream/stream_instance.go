package stream

import (
	"context"
	"fmt"
	"m3u-stream-merger/logger"
	"m3u-stream-merger/proxy"
	"m3u-stream-merger/proxy/client"
	"m3u-stream-merger/proxy/loadbalancer"
	"m3u-stream-merger/proxy/stream/buffer"
	"m3u-stream-merger/proxy/stream/config"
	"m3u-stream-merger/proxy/stream/failovers"
	"m3u-stream-merger/store"
	"m3u-stream-merger/utils"
	"strings"
	"time"
)

type StreamInstance struct {
	Cm           *store.ConcurrencyManager
	config       *config.StreamConfig
	logger       logger.Logger
	failoverProc *failovers.M3U8Processor
}

type StreamInstanceOption func(*StreamInstance)

func WithLogger(logger logger.Logger) StreamInstanceOption {
	return func(s *StreamInstance) {
		s.logger = logger
	}
}

func NewStreamInstance(
	cm *store.ConcurrencyManager,
	config *config.StreamConfig,
	opts ...StreamInstanceOption,
) (*StreamInstance, error) {
	if cm == nil {
		return nil, fmt.Errorf("concurrency manager is required")
	}

	instance := &StreamInstance{
		Cm:           cm,
		config:       config,
		failoverProc: failovers.NewM3U8Processor(&logger.DefaultLogger{}),
	}

	// Apply all options
	for _, opt := range opts {
		opt(instance)
	}

	if instance.logger == nil {
		instance.logger = &logger.DefaultLogger{}
	}

	return instance, nil
}

func (instance *StreamInstance) ProxyStream(
	ctx context.Context,
	coordinator *buffer.StreamCoordinator,
	lbResult *loadbalancer.LoadBalancerResult,
	streamClient *client.StreamClient,
	statusChan chan<- int,
) {
	startTime := time.Now()
	
	// Create a logger with correlation ID
	requestLogger := instance.logger.WithCorrelationID(ctx)
	handler := NewStreamHandler(instance.config, coordinator, requestLogger)

	requestLogger.InfoEvent().
		Str("component", "StreamInstance").
		Str("client_ip", streamClient.Request.RemoteAddr).
		Str("target_url", lbResult.URL).
		Str("lb_index", lbResult.Index).
		Str("lb_sub_index", lbResult.SubIndex).
		Int("status_code", lbResult.Response.StatusCode).
		Msg("Starting stream proxy operation")

	var result StreamResult
	if lbResult.Response.StatusCode == 206 || strings.HasSuffix(lbResult.URL, ".mp4") {
		requestLogger.InfoEvent().
			Str("component", "StreamInstance").
			Str("client_ip", streamClient.Request.RemoteAddr).
			Str("target_url", lbResult.URL).
			Int("status_code", lbResult.Response.StatusCode).
			Bool("is_vod", true).
			Msg("VOD request detected, using direct stream")
		requestLogger.WarnEvent().
			Str("component", "StreamInstance").
			Str("client_ip", streamClient.Request.RemoteAddr).
			Msg("VODs do not support shared buffer")
		result = handler.HandleDirectStream(ctx, lbResult, streamClient)
	} else {
		if _, ok := instance.Cm.Invalid.Load(lbResult.URL); !ok {
			requestLogger.DebugEvent().
				Str("component", "StreamInstance").
				Str("client_ip", streamClient.Request.RemoteAddr).
				Str("target_url", lbResult.URL).
				Msg("Using shared buffer stream handling")
			result = handler.HandleStream(ctx, lbResult, streamClient)
		} else {
			requestLogger.WarnEvent().
				Str("component", "StreamInstance").
				Str("client_ip", streamClient.Request.RemoteAddr).
				Str("target_url", lbResult.URL).
				Msg("URL marked as incompatible, returning incompatible status")
			result = StreamResult{
				Status: proxy.StatusIncompatible,
			}
		}
	}
	
	if result.Error != nil {
		if result.Status != proxy.StatusIncompatible {
			requestLogger.ErrorEvent().
				Str("component", "StreamInstance").
				Str("client_ip", streamClient.Request.RemoteAddr).
				Str("target_url", lbResult.URL).
				Int64("bytes_written", result.BytesWritten).
				Int("status", result.Status).
				Dur("duration", time.Since(startTime)).
				Err(result.Error).
				Msg("Stream handler error")
		}
	}

	if result.Status == proxy.StatusIncompatible && utils.IsAnM3U8Media(lbResult.Response) {
		if _, ok := instance.Cm.Invalid.Load(lbResult.URL); !ok {
			requestLogger.WarnEvent().
				Str("component", "StreamInstance").
				Str("client_ip", streamClient.Request.RemoteAddr).
				Str("target_url", lbResult.URL).
				Msg("Source is incompatible for M3U8, trying fallback passthrough method")
			requestLogger.WarnEvent().
				Str("component", "StreamInstance").
				Str("client_ip", streamClient.Request.RemoteAddr).
				Msg("Passthrough method will not have shared buffer, concurrency support might be unreliable")
			instance.Cm.Invalid.Store(lbResult.URL, struct{}{})
		}

		if err := instance.failoverProc.ProcessM3U8Stream(lbResult, streamClient); err != nil {
			requestLogger.ErrorEvent().
				Str("component", "StreamInstance").
				Str("client_ip", streamClient.Request.RemoteAddr).
				Str("target_url", lbResult.URL).
				Dur("duration", time.Since(startTime)).
				Err(err).
				Msg("M3U8 failover processing failed")
			statusChan <- proxy.StatusIncompatible
			return
		}

		requestLogger.InfoEvent().
			Str("component", "StreamInstance").
			Str("client_ip", streamClient.Request.RemoteAddr).
			Str("target_url", lbResult.URL).
			Dur("duration", time.Since(startTime)).
			Msg("M3U8 failover processing completed successfully")
		statusChan <- proxy.StatusM3U8Parsed
		return
	}

	if utils.IsAnM3U8Media(lbResult.Response) {
		lbResult.Response.Body.Close()
	}

	requestLogger.InfoEvent().
		Str("component", "StreamInstance").
		Str("client_ip", streamClient.Request.RemoteAddr).
		Str("target_url", lbResult.URL).
		Int64("bytes_written", result.BytesWritten).
		Int("status", result.Status).
		Dur("duration", time.Since(startTime)).
		Msg("Stream proxy operation completed")

	statusChan <- result.Status
}
