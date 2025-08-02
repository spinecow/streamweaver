package handlers

import (
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"time"

	"m3u-stream-merger/logger"
	"m3u-stream-merger/utils"
)

type PassthroughHTTPHandler struct {
	logger logger.Logger
}

func NewPassthroughHTTPHandler(logger logger.Logger) *PassthroughHTTPHandler {
	return &PassthroughHTTPHandler{
		logger: logger,
	}
}

func (h *PassthroughHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// Create a logger with correlation ID
	requestLogger := h.logger.WithCorrelationID(r.Context())

	requestLogger.InfoEvent().
		Str("component", "PassthroughHTTPHandler").
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Str("client_ip", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Msg("Processing passthrough request")

	const prefix = "/a/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		requestLogger.ErrorEvent().
			Str("component", "PassthroughHTTPHandler").
			Str("operation", "validate_path").
			Str("url_path", r.URL.Path).
			Int("status_code", http.StatusBadRequest).
			Dur("duration", time.Since(startTime)).
			Msg("Invalid URL path: missing " + prefix)
		http.Error(w, "Invalid URL provided", http.StatusBadRequest)
		return
	}

	encodedURL := r.URL.Path[len(prefix):]
	if encodedURL == "" {
		requestLogger.ErrorEvent().
			Str("component", "PassthroughHTTPHandler").
			Str("operation", "validate_encoded_url").
			Int("status_code", http.StatusBadRequest).
			Dur("duration", time.Since(startTime)).
			Msg("No encoded URL provided in the path")
		http.Error(w, "No URL provided", http.StatusBadRequest)
		return
	}

	// Measure URL decoding time
	decodeStartTime := time.Now()
	originalURLBytes, err := base64.URLEncoding.DecodeString(encodedURL)
	decodeDuration := time.Since(decodeStartTime)
	
	if err != nil {
		requestLogger.ErrorEvent().
			Str("component", "PassthroughHTTPHandler").
			Str("operation", "decode_url").
			Dur("decode_duration", decodeDuration).
			Dur("duration", time.Since(startTime)).
			Int("status_code", http.StatusBadRequest).
			Err(err).
			Msg("Failed to decode original URL")
		http.Error(w, "Failed to decode original URL", http.StatusBadRequest)
		return
	}

	originalURL := string(originalURLBytes)
	
	requestLogger.DebugEvent().
		Str("component", "PassthroughHTTPHandler").
		Str("operation", "decode_url").
		Dur("decode_duration", decodeDuration).
		Msg("URL decoded successfully")

	// Measure request creation time
	requestCreateStartTime := time.Now()
	proxyReq, err := http.NewRequest(r.Method, originalURL, r.Body)
	requestCreateDuration := time.Since(requestCreateStartTime)
	
	if err != nil {
		requestLogger.ErrorEvent().
			Str("component", "PassthroughHTTPHandler").
			Str("operation", "create_request").
			Dur("request_create_duration", requestCreateDuration).
			Dur("duration", time.Since(startTime)).
			Int("status_code", http.StatusInternalServerError).
			Err(err).
			Msg("Failed to create new request")
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}

	proxyReq = proxyReq.WithContext(r.Context())
	proxyReq.Header = r.Header.Clone()

	// Add correlation ID to the downstream request
	if correlationID, ok := utils.CorrelationIDFromRequest(r); ok {
		proxyReq.Header.Set("X-Correlation-ID", correlationID)
	}

	requestLogger.DebugEvent().
		Str("component", "PassthroughHTTPHandler").
		Str("operation", "create_request").
		Dur("request_create_duration", requestCreateDuration).
		Msg("Proxy request created successfully")

	// Measure upstream request time
	upstreamStartTime := time.Now()
	resp, err := utils.HTTPClient.Do(proxyReq)
	upstreamDuration := time.Since(upstreamStartTime)
	
	if err != nil {
		requestLogger.ErrorEvent().
			Str("component", "PassthroughHTTPHandler").
			Str("operation", "upstream_request").
			Dur("upstream_duration", upstreamDuration).
			Dur("duration", time.Since(startTime)).
			Int("status_code", http.StatusBadGateway).
			Err(err).
			Msg("Failed to fetch original URL")
		http.Error(w, "Error fetching the requested resource", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	requestLogger.InfoEvent().
		Str("component", "PassthroughHTTPHandler").
		Str("operation", "upstream_request").
		Dur("upstream_duration", upstreamDuration).
		Int("upstream_status_code", resp.StatusCode).
		Msg("Upstream request completed")

	// Measure header copying time
	headerCopyStartTime := time.Now()
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	headerCopyDuration := time.Since(headerCopyStartTime)

	w.WriteHeader(resp.StatusCode)

	// Measure response body copying time
	bodyCopyStartTime := time.Now()
	bytesWritten, err := io.Copy(w, resp.Body)
	bodyCopyDuration := time.Since(bodyCopyStartTime)
	totalDuration := time.Since(startTime)

	if err != nil {
		requestLogger.ErrorEvent().
			Str("component", "PassthroughHTTPHandler").
			Str("operation", "copy_response_body").
			Dur("body_copy_duration", bodyCopyDuration).
			Dur("header_copy_duration", headerCopyDuration).
			Dur("upstream_duration", upstreamDuration).
			Dur("decode_duration", decodeDuration).
			Dur("request_create_duration", requestCreateDuration).
			Dur("duration", totalDuration).
			Int64("bytes_written", bytesWritten).
			Int("status_code", resp.StatusCode).
			Err(err).
			Msg("Failed to write response body")
	} else {
		requestLogger.InfoEvent().
			Str("component", "PassthroughHTTPHandler").
			Str("operation", "proxy_complete").
			Dur("body_copy_duration", bodyCopyDuration).
			Dur("header_copy_duration", headerCopyDuration).
			Dur("upstream_duration", upstreamDuration).
			Dur("decode_duration", decodeDuration).
			Dur("request_create_duration", requestCreateDuration).
			Dur("duration", totalDuration).
			Int64("bytes_written", bytesWritten).
			Int("status_code", resp.StatusCode).
			Msg("Passthrough request completed successfully")
	}
}
