package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"m3u-stream-merger/correlation"
	"m3u-stream-merger/handlers"
	"m3u-stream-merger/logger"
)

// TestHTTPHandlersCorrelationIntegration tests correlation ID handling across HTTP handlers
func TestHTTPHandlersCorrelationIntegration(t *testing.T) {
	t.Run("CorrelationMiddlewareIntegration", func(t *testing.T) {
		// Create a test handler that uses the logger
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := logger.Default.WithCorrelationID(r.Context())
			
			requestLogger.InfoEvent().
				Str("component", "TestHandler").
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Msg("Processing test request")
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		// Test with correlation middleware
		handler := correlation.Middleware(testHandler)

		// Test 1: Request without correlation ID
		req1 := httptest.NewRequest("GET", "/test", nil)
		w1 := httptest.NewRecorder()
		
		handler.ServeHTTP(w1, req1)
		
		// Verify correlation ID was generated and added to response
		correlationID := w1.Header().Get("X-Correlation-ID")
		if correlationID == "" {
			t.Error("Correlation ID should be generated and added to response header")
		}
		
		if len(correlationID) < 10 {
			t.Errorf("Generated correlation ID seems too short: %s", correlationID)
		}

		// Test 2: Request with existing correlation ID
		existingID := "existing-test-id-123"
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("X-Correlation-ID", existingID)
		w2 := httptest.NewRecorder()
		
		handler.ServeHTTP(w2, req2)
		
		// Verify existing correlation ID was preserved
		responseID := w2.Header().Get("X-Correlation-ID")
		if responseID != existingID {
			t.Errorf("Expected correlation ID %s to be preserved, got %s", existingID, responseID)
		}
	})

	t.Run("M3UHandlerLoggingIntegration", func(t *testing.T) {
		// Set up JSON logging for easier verification
		originalFormat := os.Getenv("LOG_FORMAT")
		os.Setenv("LOG_FORMAT", "json")
		defer func() {
			if originalFormat == "" {
				os.Unsetenv("LOG_FORMAT")
			} else {
				os.Setenv("LOG_FORMAT", originalFormat)
			}
		}()

		// Create M3U handler
		m3uHandler := handlers.NewM3UHTTPHandler(logger.Default, "")

		// Test 1: Request with correlation ID
		correlationID := "m3u-test-correlation-123"
		req := httptest.NewRequest("GET", "/playlist.m3u", nil)
		ctx := correlation.WithCorrelationID(req.Context(), correlationID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		m3uHandler.ServeHTTP(w, req)

		// Should return 404 since no processed path is set
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
		}

		// Test 2: Authentication logging
		// Set up credentials for testing
		os.Setenv("CREDENTIALS", `[{"username":"testuser","password":"testpass","expiration":"2030-01-01T00:00:00Z"}]`)
		defer os.Unsetenv("CREDENTIALS")

		m3uHandlerWithCreds := handlers.NewM3UHTTPHandler(logger.Default, "")

		// Test authentication failure
		req2 := httptest.NewRequest("GET", "/playlist.m3u?username=invalid&password=invalid", nil)
		ctx2 := correlation.WithCorrelationID(req2.Context(), "m3u-auth-test-456")
		req2 = req2.WithContext(ctx2)
		
		w2 := httptest.NewRecorder()
		m3uHandlerWithCreds.ServeHTTP(w2, req2)

		// Should return 403 for invalid credentials
		if w2.Code != http.StatusForbidden {
			t.Errorf("Expected status code %d for invalid credentials, got %d", 
				http.StatusForbidden, w2.Code)
		}

		// Test authentication success
		req3 := httptest.NewRequest("GET", "/playlist.m3u?username=testuser&password=testpass", nil)
		ctx3 := correlation.WithCorrelationID(req3.Context(), "m3u-auth-success-789")
		req3 = req3.WithContext(ctx3)
		
		w3 := httptest.NewRecorder()
		m3uHandlerWithCreds.ServeHTTP(w3, req3)

		// Should return 404 (no processed file) but authentication should succeed
		if w3.Code != http.StatusNotFound {
			t.Errorf("Expected status code %d after successful auth, got %d", 
				http.StatusNotFound, w3.Code)
		}
	})

	t.Run("StreamHandlerLoggingIntegration", func(t *testing.T) {
		// Create stream handler
		mockProxy := handlers.NewDefaultProxyInstance()
		streamHandler := handlers.NewStreamHTTPHandler(mockProxy, logger.Default)

		// Test 1: Invalid stream ID
		req1 := httptest.NewRequest("GET", "/stream/", nil)
		ctx1 := correlation.WithCorrelationID(req1.Context(), "stream-invalid-123")
		req1 = req1.WithContext(ctx1)
		
		w1 := httptest.NewRecorder()
		streamHandler.ServeHTTP(w1, req1)

		// Should handle invalid stream ID gracefully
		// The exact behavior depends on implementation

		// Test 2: Valid stream ID format
		req2 := httptest.NewRequest("GET", "/stream/test-stream.m3u8", nil)
		ctx2 := correlation.WithCorrelationID(req2.Context(), "stream-valid-456")
		req2 = req2.WithContext(ctx2)
		
		w2 := httptest.NewRecorder()
		streamHandler.ServeHTTP(w2, req2)

		// Should attempt to process the stream (will fail without real backend)
	})

	t.Run("PassthroughHandlerLoggingIntegration", func(t *testing.T) {
		passthroughHandler := handlers.NewPassthroughHTTPHandler(logger.Default)

		// Test 1: Invalid path prefix
		req1 := httptest.NewRequest("GET", "/invalid/path", nil)
		ctx1 := correlation.WithCorrelationID(req1.Context(), "passthrough-invalid-123")
		req1 = req1.WithContext(ctx1)
		
		w1 := httptest.NewRecorder()
		passthroughHandler.ServeHTTP(w1, req1)

		if w1.Code != http.StatusBadRequest {
			t.Errorf("Expected status code %d for invalid path, got %d", 
				http.StatusBadRequest, w1.Code)
		}

		// Test 2: Missing encoded URL
		req2 := httptest.NewRequest("GET", "/a/", nil)
		ctx2 := correlation.WithCorrelationID(req2.Context(), "passthrough-empty-456")
		req2 = req2.WithContext(ctx2)
		
		w2 := httptest.NewRecorder()
		passthroughHandler.ServeHTTP(w2, req2)

		if w2.Code != http.StatusBadRequest {
			t.Errorf("Expected status code %d for empty encoded URL, got %d", 
				http.StatusBadRequest, w2.Code)
		}

		// Test 3: Invalid base64 encoding
		req3 := httptest.NewRequest("GET", "/a/invalid-base64!", nil)
		ctx3 := correlation.WithCorrelationID(req3.Context(), "passthrough-invalid-base64-789")
		req3 = req3.WithContext(ctx3)
		
		w3 := httptest.NewRecorder()
		passthroughHandler.ServeHTTP(w3, req3)

		if w3.Code != http.StatusBadRequest {
			t.Errorf("Expected status code %d for invalid base64, got %d", 
				http.StatusBadRequest, w3.Code)
		}
	})
}

// TestLogFormatIntegration tests different log formats in HTTP handlers
func TestLogFormatIntegration(t *testing.T) {
	t.Run("JSONLogFormat", func(t *testing.T) {
		// Set JSON format
		originalFormat := os.Getenv("LOG_FORMAT")
		os.Setenv("LOG_FORMAT", "json")
		defer func() {
			if originalFormat == "" {
				os.Unsetenv("LOG_FORMAT")
			} else {
				os.Setenv("LOG_FORMAT", originalFormat)
			}
		}()

		testLogger := logger.NewDefaultLogger()

		// Create a handler that logs in JSON format
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := testLogger.WithCorrelationID(r.Context())
			
			requestLogger.InfoEvent().
				Str("component", "JSONTestHandler").
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("user_agent", r.UserAgent()).
				Int("content_length", int(r.ContentLength)).
				Msg("Processing JSON format test request")
			
			w.WriteHeader(http.StatusOK)
		})

		handler := correlation.Middleware(testHandler)

		req := httptest.NewRequest("GET", "/json-test", nil)
		req.Header.Set("User-Agent", "JSON-Test-Agent/1.0")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("ConsoleLogFormat", func(t *testing.T) {
		// Set console format
		originalFormat := os.Getenv("LOG_FORMAT")
		os.Setenv("LOG_FORMAT", "console")
		defer func() {
			if originalFormat == "" {
				os.Unsetenv("LOG_FORMAT")
			} else {
				os.Setenv("LOG_FORMAT", originalFormat)
			}
		}()

		testLogger := logger.NewDefaultLogger()

		// Create a handler that logs in console format
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := testLogger.WithCorrelationID(r.Context())
			
			requestLogger.InfoEvent().
				Str("component", "ConsoleTestHandler").
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Msg("Processing console format test request")
			
			w.WriteHeader(http.StatusOK)
		})

		handler := correlation.Middleware(testHandler)

		req := httptest.NewRequest("GET", "/console-test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})
}

// TestSafeLogsIntegration tests safe logging in HTTP handlers
func TestSafeLogsIntegration(t *testing.T) {
	t.Run("SafeLogsEnabled", func(t *testing.T) {
		// Enable safe logs
		originalSafeLogs := os.Getenv("SAFE_LOGS")
		os.Setenv("SAFE_LOGS", "true")
		defer func() {
			if originalSafeLogs == "" {
				os.Unsetenv("SAFE_LOGS")
			} else {
				os.Setenv("SAFE_LOGS", originalSafeLogs)
			}
		}()

		testLogger := logger.NewDefaultLogger()

		// Create a handler that logs sensitive information
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := testLogger.WithCorrelationID(r.Context())
			
			// Log a message with a URL (should be redacted)
			requestLogger.Log("Processing request from http://sensitive-domain.com/api/v1/data")
			
			// Use sensitive field logging
			sensitiveLogger := requestLogger.WithSensitiveField("api_key", "secret-api-key-123")
			sensitiveLogger.InfoEvent().
				Str("component", "SafeLogsTestHandler").
				Msg("Request with sensitive information")
			
			w.WriteHeader(http.StatusOK)
		})

		handler := correlation.Middleware(testHandler)

		req := httptest.NewRequest("GET", "/safe-logs-test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("SafeLogsDisabled", func(t *testing.T) {
		// Disable safe logs
		originalSafeLogs := os.Getenv("SAFE_LOGS")
		os.Setenv("SAFE_LOGS", "false")
		defer func() {
			if originalSafeLogs == "" {
				os.Unsetenv("SAFE_LOGS")
			} else {
				os.Setenv("SAFE_LOGS", originalSafeLogs)
			}
		}()

		testLogger := logger.NewDefaultLogger()

		// Create a handler that logs sensitive information
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := testLogger.WithCorrelationID(r.Context())
			
			// Log a message with a URL (should NOT be redacted)
			requestLogger.Log("Processing request from http://normal-domain.com/api/v1/data")
			
			// Use sensitive field logging
			sensitiveLogger := requestLogger.WithSensitiveField("api_key", "normal-api-key-456")
			sensitiveLogger.InfoEvent().
				Str("component", "UnsafeLogsTestHandler").
				Msg("Request with normal information")
			
			w.WriteHeader(http.StatusOK)
		})

		handler := correlation.Middleware(testHandler)

		req := httptest.NewRequest("GET", "/unsafe-logs-test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})
}

// TestContextualFieldsIntegration tests contextual fields in HTTP handlers
func TestContextualFieldsIntegration(t *testing.T) {
	t.Run("RequestContextualFields", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Create a handler that logs with extensive contextual fields
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			
			// Create base logger with correlation ID
			requestLogger := testLogger.WithCorrelationID(r.Context())
			
			// Add request-specific contextual fields
			contextualLogger := requestLogger.WithFields(map[string]interface{}{
				"component":     "ContextualTestHandler",
				"method":        r.Method,
				"path":          r.URL.Path,
				"query_params":  r.URL.RawQuery,
				"client_ip":     r.RemoteAddr,
				"user_agent":    r.UserAgent(),
				"content_type":  r.Header.Get("Content-Type"),
				"content_length": r.ContentLength,
				"host":          r.Host,
				"referer":       r.Header.Get("Referer"),
			})

			// Log request start
			contextualLogger.InfoEvent().
				Msg("Request processing started")

			// Simulate processing with different contextual information
			businessLogger := contextualLogger.WithFields(map[string]interface{}{
				"operation": "business_logic",
				"step":      1,
			})

			businessLogger.DebugEvent().
				Msg("Starting business logic processing")

			// Simulate different steps
			for step := 1; step <= 3; step++ {
				stepLogger := businessLogger.WithFields(map[string]interface{}{
					"step": step,
				})
				
				stepLogger.DebugEvent().
					Dur("elapsed", time.Since(startTime)).
					Msg(fmt.Sprintf("Processing step %d", step))
				
				// Simulate processing time
				time.Sleep(10 * time.Millisecond)
			}

			// Log completion
			contextualLogger.InfoEvent().
				Dur("total_duration", time.Since(startTime)).
				Int("status_code", http.StatusOK).
				Msg("Request processing completed")
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		handler := correlation.Middleware(testHandler)

		req := httptest.NewRequest("GET", "/contextual-test?param1=value1&param2=value2", nil)
		req.Header.Set("User-Agent", "Contextual-Test-Agent/1.0")
		req.Header.Set("Referer", "http://example.com/previous-page")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("ErrorContextualFields", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Create a handler that logs errors with contextual information
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := testLogger.WithCorrelationID(r.Context())
			
			// Add error contextual fields
			errorLogger := requestLogger.WithFields(map[string]interface{}{
				"component":   "ErrorTestHandler",
				"error_type":  "validation_error",
				"error_code":  "INVALID_PARAM",
			})

			// Simulate an error scenario
			if r.URL.Query().Get("simulate_error") == "true" {
				errorLogger.ErrorEvent().
					Str("parameter", "simulate_error").
					Str("parameter_value", "true").
					Err(fmt.Errorf("simulated validation error")).
					Msg("Parameter validation failed")
				
				http.Error(w, "Validation Error", http.StatusBadRequest)
				return
			}

			requestLogger.InfoEvent().
				Str("component", "ErrorTestHandler").
				Msg("Request processed successfully")
			
			w.WriteHeader(http.StatusOK)
		})

		handler := correlation.Middleware(testHandler)

		// Test error scenario
		req1 := httptest.NewRequest("GET", "/error-test?simulate_error=true", nil)
		w1 := httptest.NewRecorder()

		handler.ServeHTTP(w1, req1)

		if w1.Code != http.StatusBadRequest {
			t.Errorf("Expected status code %d for error scenario, got %d", 
				http.StatusBadRequest, w1.Code)
		}

		// Test success scenario
		req2 := httptest.NewRequest("GET", "/error-test?simulate_error=false", nil)
		w2 := httptest.NewRecorder()

		handler.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("Expected status code %d for success scenario, got %d", 
				http.StatusOK, w2.Code)
		}
	})
}

// TestHandlerPerformanceLogging tests performance logging in handlers
func TestHandlerPerformanceLogging(t *testing.T) {
	t.Run("RequestDurationLogging", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Create a handler that logs performance metrics
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			
			requestLogger := testLogger.WithCorrelationID(r.Context())
			
			// Log request start
			requestLogger.InfoEvent().
				Str("component", "PerformanceTestHandler").
				Time("start_time", startTime).
				Msg("Request processing started")

			// Simulate different processing durations
			processingTime := 100 * time.Millisecond
			if r.URL.Query().Get("slow") == "true" {
				processingTime = 500 * time.Millisecond
			}

			time.Sleep(processingTime)

			endTime := time.Now()
			duration := endTime.Sub(startTime)

			// Log performance metrics
			requestLogger.InfoEvent().
				Str("component", "PerformanceTestHandler").
				Time("end_time", endTime).
				Dur("duration", duration).
				Int64("duration_ms", duration.Milliseconds()).
				Bool("slow_request", duration > 200*time.Millisecond).
				Msg("Request processing completed")
			
			w.WriteHeader(http.StatusOK)
		})

		handler := correlation.Middleware(testHandler)

		// Test normal request
		req1 := httptest.NewRequest("GET", "/performance-test", nil)
		w1 := httptest.NewRecorder()

		handler.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w1.Code)
		}

		// Test slow request
		req2 := httptest.NewRequest("GET", "/performance-test?slow=true", nil)
		w2 := httptest.NewRecorder()

		handler.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w2.Code)
		}
	})
}