package handlers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"m3u-stream-merger/logger"
	"m3u-stream-merger/proxy"
	"m3u-stream-merger/proxy/client"
	"m3u-stream-merger/proxy/stream/failovers"
	"m3u-stream-merger/sourceproc"
	"m3u-stream-merger/utils"
)

type StreamHTTPHandler struct {
	manager ProxyInstance
	logger  logger.Logger
}

func NewStreamHTTPHandler(manager ProxyInstance, logger logger.Logger) *StreamHTTPHandler {
	return &StreamHTTPHandler{
		manager: manager,
		logger:  logger,
	}
}

func (h *StreamHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	streamClient := client.NewStreamClient(w, r)

	h.handleStream(r.Context(), streamClient)
}

func (h *StreamHTTPHandler) ServeSegmentHTTP(w http.ResponseWriter, r *http.Request) {
	streamClient := client.NewStreamClient(w, r)

	h.handleSegmentStream(streamClient)
}

func (h *StreamHTTPHandler) extractStreamURL(urlPath string) string {
	base := path.Base(urlPath)
	parts := strings.Split(base, ".")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimPrefix(parts[0], "/")
}

func (h *StreamHTTPHandler) handleStream(ctx context.Context, streamClient *client.StreamClient) {
	r := streamClient.Request
	startTime := time.Now()

	// Create a logger with correlation ID
	requestLogger := h.logger.WithCorrelationID(r.Context())

	streamURL := h.extractStreamURL(r.URL.Path)
	if streamURL == "" {
		// Get channel name from stream URL
		streamInfo, _ := sourceproc.DecodeSlug(streamURL)
		channelName := ""
		if streamInfo != nil {
			channelName = streamInfo.Title
		}
	
		requestLogger.ErrorEvent().
			Str("component", "StreamHTTPHandler").
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Str("client_ip", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Str("url_path", r.URL.Path).
			Str("channel_name", channelName).
			Msg("Invalid m3uID for request")
		return
	}

	// Get channel name from stream URL
	streamInfo, _ := sourceproc.DecodeSlug(streamURL)
	channelName := ""
	if streamInfo != nil {
		channelName = streamInfo.Title
	}

	requestLogger.InfoEvent().
		Str("component", "StreamHTTPHandler").
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Str("client_ip", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Str("stream_id", streamURL).
		Str("channel_name", channelName).
		Msg("Processing stream request")

	// Try to get the actual URL from an existing coordinator if available
	actualURL := ""
	// We'll get the actual URL from the load balancer result later
	coordinator := h.manager.GetStreamRegistry().GetOrCreateCoordinator(streamURL, actualURL)

	for {
		lbResult := coordinator.GetWriterLBResult()
		var err error
		if lbResult == nil {
			requestLogger.DebugEvent().
				Str("component", "StreamHTTPHandler").
				Str("stream_id", streamURL).
				Str("channel_name", channelName).
				Str("client_ip", r.RemoteAddr).
				Msg("No existing shared buffer found, executing load balancer")
			
			// Measure load balancer execution time
			lbStartTime := time.Now()
			lbResult, err = h.manager.LoadBalancer(ctx, r)
			lbDuration := time.Since(lbStartTime)
			
			if err != nil {
				requestLogger.ErrorEvent().
					Str("component", "StreamHTTPHandler").
					Str("operation", "load_balancer").
					Str("path", r.URL.Path).
					Str("stream_id", streamURL).
					Str("channel_name", channelName).
					Dur("lb_duration", lbDuration).
					Err(err).
					Msg("Load balancer error")
				return
			}
			
			requestLogger.InfoEvent().
				Str("component", "StreamHTTPHandler").
				Str("operation", "load_balancer").
				Str("stream_id", streamURL).
				Str("channel_name", channelName).
				Str("selected_url", lbResult.URL).
				Dur("lb_duration", lbDuration).
				Msg("Load balancer execution completed")
		} else {
			// We have an existing coordinator, check if it has the same actual URL
			if lbResult != nil && lbResult.URL != "" {
				if coordinator.GetActualURL() == "" {
					// Set the actual URL on the coordinator if it's not set yet
					coordinator.SetActualURL(lbResult.URL)
				} else if coordinator.GetActualURL() != lbResult.URL {
					// The existing coordinator has a different actual URL, try to find or create a coordinator for this URL
					requestLogger.InfoEvent().
						Str("component", "StreamHTTPHandler").
						Str("stream_id", streamURL).
						Str("channel_name", channelName).
						Str("existing_url", coordinator.GetActualURL()).
						Str("new_url", lbResult.URL).
						Msg("Existing coordinator has different actual URL, finding/creating new coordinator")
					coordinator = h.manager.GetStreamRegistry().GetOrCreateCoordinator(streamURL, lbResult.URL)
				} else {
					if _, ok := h.manager.GetConcurrencyManager().Invalid.Load(lbResult.URL); !ok {
						requestLogger.InfoEvent().
							Str("component", "StreamHTTPHandler").
							Str("stream_id", streamURL).
							Str("channel_name", channelName).
							Str("actual_url", lbResult.URL).
							Msg("Existing shared buffer found with same actual URL")
					}
				}
			} else {
				if _, ok := h.manager.GetConcurrencyManager().Invalid.Load(lbResult.URL); !ok {
					requestLogger.InfoEvent().
						Str("component", "StreamHTTPHandler").
						Str("stream_id", streamURL).
						Str("channel_name", channelName).
						Msg("Existing shared buffer found")
				}
			}
		}

		exitStatus := make(chan int)
		requestLogger.InfoEvent().
			Str("component", "StreamHTTPHandler").
			Str("operation", "proxy_stream").
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Str("client_ip", r.RemoteAddr).
			Str("stream_id", streamURL).
			Str("channel_name", channelName).
			Str("target_url", lbResult.URL).
			Msg("Starting stream proxy")

		// Measure stream proxy initialization time
		proxyInitStartTime := time.Now()
		proxyCtx, cancel := context.WithCancel(ctx)
		go func() {
			defer cancel()
			h.manager.ProxyStream(proxyCtx, coordinator, lbResult, streamClient, exitStatus)
		}()
		proxyInitDuration := time.Since(proxyInitStartTime)
		
		requestLogger.DebugEvent().
			Str("component", "StreamHTTPHandler").
			Str("operation", "proxy_stream_init").
			Str("stream_id", streamURL).
			Str("channel_name", channelName).
			Dur("proxy_init_duration", proxyInitDuration).
			Msg("Stream proxy initialized")

		select {
		case <-ctx.Done():
			requestLogger.InfoEvent().
				Str("component", "StreamHTTPHandler").
				Str("client_ip", r.RemoteAddr).
				Str("stream_id", streamURL).
				Str("channel_name", channelName).
				Dur("duration", time.Since(startTime)).
				Msg("Client has closed the stream")
			return
		case code := <-exitStatus:
			if h.handleExitCodeWithLogger(code, r, requestLogger) {
				requestLogger.InfoEvent().
					Str("component", "StreamHTTPHandler").
					Str("client_ip", r.RemoteAddr).
					Str("stream_id", streamURL).
					Str("channel_name", channelName).
					Int("exit_code", code).
					Dur("duration", time.Since(startTime)).
					Msg("Stream request completed")
				return
			}
			// Otherwise, retry with a new lbResult.
		}

		select {
		case <-ctx.Done():
			requestLogger.InfoEvent().
				Str("component", "StreamHTTPHandler").
				Str("client_ip", r.RemoteAddr).
				Str("stream_id", streamURL).
				Str("channel_name", channelName).
				Dur("duration", time.Since(startTime)).
				Msg("Client has closed the stream during retry wait")
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (h *StreamHTTPHandler) handleExitCodeWithLogger(code int, r *http.Request, requestLogger logger.Logger) bool {
	// Get channel name from stream URL
	streamURL := h.extractStreamURL(r.URL.Path)
	streamInfo, _ := sourceproc.DecodeSlug(streamURL)
	channelName := ""
	if streamInfo != nil {
		channelName = streamInfo.Title
	}
	
	switch code {
	case proxy.StatusIncompatible:
		requestLogger.ErrorEvent().
			Str("component", "StreamHTTPHandler").
			Str("method", r.Method).
			Str("remote_addr", r.RemoteAddr).
			Str("channel_name", channelName).
			Msg("Finished handling M3U8 request but failed to parse contents.")
		fallthrough
	case proxy.StatusEOF:
		fallthrough
	case proxy.StatusServerError:
		requestLogger.InfoEvent().
			Str("component", "StreamHTTPHandler").
			Str("method", r.Method).
			Str("client_ip", r.RemoteAddr).
			Str("channel_name", channelName).
			Int("exit_code", code).
			Msg("Retrying other servers")
		return false
	case proxy.StatusM3U8Parsed:
		requestLogger.DebugEvent().
			Str("component", "StreamHTTPHandler").
			Str("method", r.Method).
			Str("client_ip", r.RemoteAddr).
			Str("channel_name", channelName).
			Msg("Finished handling M3U8 request")
		return true
	case proxy.StatusM3U8ParseError:
		requestLogger.ErrorEvent().
			Str("component", "StreamHTTPHandler").
			Str("method", r.Method).
			Str("remote_addr", r.RemoteAddr).
			Str("channel_name", channelName).
			Msg("Finished handling M3U8 request but failed to parse contents")
		return false
	default:
		requestLogger.InfoEvent().
			Str("component", "StreamHTTPHandler").
			Str("method", r.Method).
			Str("client_ip", r.RemoteAddr).
			Str("channel_name", channelName).
			Int("exit_code", code).
			Msg("Unable to write to client. Assuming stream has been closed")
		return true
	}
}

func (h *StreamHTTPHandler) handleSegmentStream(streamClient *client.StreamClient) {
	r := streamClient.Request
	startTime := time.Now()

	// Create a logger with correlation ID
	requestLogger := h.logger.WithCorrelationID(r.Context())
	
	// Get channel name from stream URL
	streamId := h.extractStreamURL(r.URL.Path)
	streamInfo, _ := sourceproc.DecodeSlug(streamId)
	channelName := ""
	if streamInfo != nil {
		channelName = streamInfo.Title
	}

	requestLogger.DebugEvent().
		Str("component", "StreamHTTPHandler").
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Str("client_ip", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Str("url_path", r.URL.Path).
		Str("channel_name", channelName).
		Msg("Received segment request")

	if streamId == "" {
		requestLogger.ErrorEvent().
			Str("component", "StreamHTTPHandler").
			Str("remote_addr", r.RemoteAddr).
			Str("url_path", r.URL.Path).
			Str("channel_name", channelName).
			Msg("Invalid m3uID for request")
		return
	}

	segment, err := failovers.ParseSegmentId(streamId)
	if err != nil {
		requestLogger.ErrorEvent().
			Str("component", "StreamHTTPHandler").
			Str("remote_addr", r.RemoteAddr).
			Str("url_path", r.URL.Path).
			Str("channel_name", channelName).
			Err(err).
			Msg("Segment parsing error")
		_ = streamClient.WriteHeader(http.StatusInternalServerError)
		_, _ = streamClient.Write([]byte(fmt.Sprintf("Segment parsing error: %v", err)))
		return
	}

	resp, err := utils.HTTPClient.Get(segment.URL)
	if err != nil {
		requestLogger.ErrorEvent().
			Str("component", "StreamHTTPHandler").
			Str("client_ip", r.RemoteAddr).
			Str("segment_url", segment.URL).
			Str("channel_name", channelName).
			Dur("duration", time.Since(startTime)).
			Err(err).
			Msg("Failed to fetch segment URL")
		_ = streamClient.WriteHeader(http.StatusInternalServerError)
		_, _ = streamClient.Write([]byte(fmt.Sprintf("Failed to fetch URL: %v", err)))
		return
	}
	defer resp.Body.Close()

	requestLogger.InfoEvent().
		Str("component", "StreamHTTPHandler").
		Str("client_ip", r.RemoteAddr).
		Str("segment_url", segment.URL).
		Str("channel_name", channelName).
		Int("status_code", resp.StatusCode).
		Dur("fetch_duration", time.Since(startTime)).
		Msg("Successfully fetched segment")

	for key, values := range resp.Header {
		for _, value := range values {
			streamClient.Header().Add(key, value)
		}
	}

	_ = streamClient.WriteHeader(resp.StatusCode)

	bytesWritten, err := io.Copy(streamClient, resp.Body)
	totalDuration := time.Since(startTime)
	
	if err != nil {
		if isBrokenPipe(err) {
			requestLogger.DebugEvent().
				Str("component", "StreamHTTPHandler").
				Str("client_ip", r.RemoteAddr).
				Str("segment_url", segment.URL).
				Str("channel_name", channelName).
				Int64("bytes_written", bytesWritten).
				Dur("duration", totalDuration).
				Err(err).
				Msg("Client disconnected (broken pipe)")
		} else {
			requestLogger.ErrorEvent().
				Str("component", "StreamHTTPHandler").
				Str("client_ip", r.RemoteAddr).
				Str("segment_url", segment.URL).
				Str("channel_name", channelName).
				Int64("bytes_written", bytesWritten).
				Dur("duration", totalDuration).
				Err(err).
				Msg("Error copying response body")
		}
	} else {
		requestLogger.InfoEvent().
			Str("component", "StreamHTTPHandler").
			Str("client_ip", r.RemoteAddr).
			Str("segment_url", segment.URL).
			Str("channel_name", channelName).
			Int("status_code", resp.StatusCode).
			Int64("bytes_written", bytesWritten).
			Dur("duration", totalDuration).
			Msg("Segment request completed successfully")
	}
}

func isBrokenPipe(err error) bool {
	if err == nil {
		return false
	}

	if opErr, ok := err.(*net.OpError); ok {
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			errMsg := sysErr.Err.Error()
			return strings.Contains(errMsg, "broken pipe") ||
				strings.Contains(errMsg, "connection reset by peer")
		}
		errMsg := opErr.Err.Error()
		return strings.Contains(errMsg, "broken pipe") ||
			strings.Contains(errMsg, "connection reset by peer")
	}

	return strings.Contains(err.Error(), "broken pipe") ||
		strings.Contains(err.Error(), "connection reset by peer")
}
