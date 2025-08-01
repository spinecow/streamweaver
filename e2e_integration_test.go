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

// TestCompleteEndToEndIntegration tests complete request flows through the system with enhanced logging
func TestCompleteEndToEndIntegration(t *testing.T) {
	t.Run("CompleteRequestFlowWithCorrelation", func(t *testing.T) {
		// Set up environment for structured logging
		originalFormat := os.Getenv("LOG_FORMAT")
		os.Setenv("LOG_FORMAT", "json")
		defer func() {
			if originalFormat == "" {
				os.Unsetenv("LOG_FORMAT")
			} else {
				os.Setenv("LOG_FORMAT", originalFormat)
			}
		}()

		// Create a complete request flow with correlation ID propagation
		correlationID := "e2e-complete-flow-" + fmt.Sprintf("%d", time.Now().Unix())
		
		// Step 1: Create a comprehensive handler that simulates real application flow
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			
			// Step 2: Extract correlation ID and create logger
			requestLogger := logger.Default.WithCorrelationID(r.Context())
			
			// Step 3: Log request processing start with comprehensive context
			requestLogger.InfoEvent().
				Str("component", "EndToEndHandler").
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("query", r.URL.RawQuery).
				Str("user_agent", r.UserAgent()).
				Str("client_ip", r.RemoteAddr).
				Str("host", r.Host).
				Str("referer", r.Header.Get("Referer")).
				Time("start_time", startTime).
				Msg("Processing end-to-end test request")
			
			// Step 4: Simulate authentication phase with contextual logging
			authLogger := requestLogger.WithFields(map[string]interface{}{
				"phase":     "authentication",
				"operation": "user_validation",
			})
			
			authLogger.DebugEvent().
				Msg("Starting authentication phase")
			
			// Simulate authentication logic
			username := r.Header.Get("X-Username")
			if username == "" {
				authLogger.WarnEvent().
					Str("error_type", "missing_credentials").
					Msg("Authentication failed: missing username")
				
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			
			authLogger.InfoEvent().
				Str("username", username).
				Dur("auth_duration", 10*time.Millisecond).
				Msg("Authentication successful")
			
			// Step 5: Simulate business logic with multiple phases
			businessLogger := requestLogger.WithFields(map[string]interface{}{
				"phase":    "business_logic",
				"username": username,
			})
			
			// Phase 1: Data validation
			validationLogger := businessLogger.WithFields(map[string]interface{}{
				"operation": "data_validation",
				"step":      1,
			})
			
			validationLogger.DebugEvent().
				Msg("Starting data validation")
			
			if r.URL.Query().Get("invalid_data") == "true" {
				validationLogger.ErrorEvent().
					Str("validation_error", "invalid_parameter").
					Str("parameter", "invalid_data").
					Msg("Data validation failed")
				
				http.Error(w, "Invalid data", http.StatusBadRequest)
				return
			}
			
			validationLogger.InfoEvent().
				Dur("validation_duration", 5*time.Millisecond).
				Msg("Data validation completed")
			
			// Phase 2: Resource processing
			processingLogger := businessLogger.WithFields(map[string]interface{}{
				"operation": "resource_processing",
				"step":      2,
			})
			
			processingLogger.DebugEvent().
				Msg("Starting resource processing")
			
			// Simulate processing time based on query parameter
			processingTime := 50 * time.Millisecond
			if r.URL.Query().Get("slow_processing") == "true" {
				processingTime = 200 * time.Millisecond
			}
			
			time.Sleep(processingTime)
			
			processingLogger.InfoEvent().
				Dur("processing_duration", processingTime).
				Bool("slow_processing", processingTime > 100*time.Millisecond).
				Msg("Resource processing completed")
			
			// Phase 3: Response preparation
			responseLogger := businessLogger.WithFields(map[string]interface{}{
				"operation": "response_preparation",
				"step":      3,
			})
			
			responseLogger.DebugEvent().
				Msg("Preparing response")
			
			// Simulate different response types
			responseType := r.URL.Query().Get("response_type")
			var statusCode int
			var responseBody string
			
			switch responseType {
			case "error":
				statusCode = http.StatusInternalServerError
				responseBody = "Internal Server Error"
				responseLogger.ErrorEvent().
					Str("response_type", responseType).
					Int("status_code", statusCode).
					Msg("Error response prepared")
			case "not_found":
				statusCode = http.StatusNotFound
				responseBody = "Resource Not Found"
				responseLogger.WarnEvent().
					Str("response_type", responseType).
					Int("status_code", statusCode).
					Msg("Not found response prepared")
			default:
				statusCode = http.StatusOK
				responseBody = "Success"
				responseLogger.InfoEvent().
					Str("response_type", "success").
					Int("status_code", statusCode).
					Msg("Success response prepared")
			}
			
			// Step 6: Log completion with comprehensive metrics
			totalDuration := time.Since(startTime)
			requestLogger.InfoEvent().
				Str("component", "EndToEndHandler").
				Str("username", username).
				Int("status_code", statusCode).
				Dur("total_duration", totalDuration).
				Time("end_time", time.Now()).
				Int("response_size", len(responseBody)).
				Bool("success", statusCode < 400).
				Msg("Request processing completed")
			
			w.WriteHeader(statusCode)
			w.Write([]byte(responseBody))
		})

		// Wrap with correlation middleware
		handler := correlation.Middleware(testHandler)

		// Test successful flow
		t.Run("SuccessfulFlow", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/e2e-test?param1=value1", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-success")
			req.Header.Set("X-Username", "testuser")
			req.Header.Set("User-Agent", "E2E-Test-Agent/1.0")
			req.Header.Set("Referer", "http://example.com/previous")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			// Verify correlation ID was preserved
			if w.Header().Get("X-Correlation-ID") != correlationID+"-success" {
				t.Errorf("Correlation ID not preserved in response")
			}
		})

		// Test authentication failure flow
		t.Run("AuthenticationFailureFlow", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/e2e-test", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-auth-fail")
			// No username header - should fail authentication
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
			}
		})

		// Test validation error flow
		t.Run("ValidationErrorFlow", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/e2e-test?invalid_data=true", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-validation-error")
			req.Header.Set("X-Username", "testuser")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})

		// Test slow processing flow
		t.Run("SlowProcessingFlow", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/e2e-test?slow_processing=true", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-slow")
			req.Header.Set("X-Username", "testuser")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}
		})

		// Test error response flow
		t.Run("ErrorResponseFlow", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/e2e-test?response_type=error", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-error")
			req.Header.Set("X-Username", "testuser")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
			}
		})
	})

	t.Run("M3UHandlerEndToEndFlow", func(t *testing.T) {
		// Test complete M3U handler flow with authentication and logging
		correlationID := "e2e-m3u-flow-" + fmt.Sprintf("%d", time.Now().Unix())
		
		// Set up credentials for testing
		originalCreds := os.Getenv("CREDENTIALS")
		os.Setenv("CREDENTIALS", `[{"username":"e2euser","password":"e2epass","expiration":"2030-01-01T00:00:00Z"}]`)
		defer func() {
			if originalCreds == "" {
				os.Unsetenv("CREDENTIALS")
			} else {
				os.Setenv("CREDENTIALS", originalCreds)
			}
		}()

		// Create M3U handler (no processed file for testing)
		m3uHandler := handlers.NewM3UHTTPHandler(logger.Default, "")
		handler := correlation.Middleware(m3uHandler)

		// Test authentication scenarios
		t.Run("M3UAuthenticationSuccess", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/playlist.m3u?username=e2euser&password=e2epass", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-auth-success")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Should return 404 (no processed file) but authentication should succeed
			if w.Code != http.StatusNotFound {
				t.Errorf("Expected status %d after successful auth, got %d", 
					http.StatusNotFound, w.Code)
			}
			
			if w.Header().Get("X-Correlation-ID") != correlationID+"-auth-success" {
				t.Error("Correlation ID not preserved in M3U response")
			}
		})

		t.Run("M3UAuthenticationFailure", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/playlist.m3u?username=invalid&password=invalid", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-auth-fail")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected status %d for invalid credentials, got %d", 
					http.StatusForbidden, w.Code)
			}
		})

		t.Run("M3UMissingCredentials", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/playlist.m3u", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-no-creds")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected status %d for missing credentials, got %d", 
					http.StatusForbidden, w.Code)
			}
		})
	})

	t.Run("PassthroughHandlerEndToEndFlow", func(t *testing.T) {
		// Test complete passthrough handler flow
		correlationID := "e2e-passthrough-" + fmt.Sprintf("%d", time.Now().Unix())
		
		passthroughHandler := handlers.NewPassthroughHTTPHandler(logger.Default)
		handler := correlation.Middleware(passthroughHandler)

		t.Run("PassthroughInvalidPath", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/invalid/path", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-invalid-path")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d for invalid path, got %d", 
					http.StatusBadRequest, w.Code)
			}
			
			if w.Header().Get("X-Correlation-ID") != correlationID+"-invalid-path" {
				t.Error("Correlation ID not preserved in passthrough response")
			}
		})

		t.Run("PassthroughInvalidBase64", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/a/invalid-base64!", nil)
			req.Header.Set("X-Correlation-ID", correlationID+"-invalid-base64")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d for invalid base64, got %d", 
					http.StatusBadRequest, w.Code)
			}
		})
	})
}

// TestEnvironmentBasedEndToEnd tests end-to-end flows with different environment configurations
func TestEnvironmentBasedEndToEnd(t *testing.T) {
	t.Run("JSONLoggingEndToEnd", func(t *testing.T) {
		// Test complete flow with JSON logging enabled
		originalFormat := os.Getenv("LOG_FORMAT")
		originalLevel := os.Getenv("LOG_LEVEL")
		originalSafeLogs := os.Getenv("SAFE_LOGS")
		
		os.Setenv("LOG_FORMAT", "json")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("SAFE_LOGS", "true")
		
		defer func() {
			if originalFormat == "" {
				os.Unsetenv("LOG_FORMAT")
			} else {
				os.Setenv("LOG_FORMAT", originalFormat)
			}
			if originalLevel == "" {
				os.Unsetenv("LOG_LEVEL")
			} else {
				os.Setenv("LOG_LEVEL", originalLevel)
			}
			if originalSafeLogs == "" {
				os.Unsetenv("SAFE_LOGS")
			} else {
				os.Setenv("SAFE_LOGS", originalSafeLogs)
			}
		}()

		correlationID := "env-json-test-" + fmt.Sprintf("%d", time.Now().Unix())
		
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := logger.Default.WithCorrelationID(r.Context())
			
			// Log with URL that should be redacted due to SAFE_LOGS=true
			requestLogger.Log("Processing request from http://sensitive-domain.com/api")
			
			// Use sensitive field logging
			sensitiveLogger := requestLogger.WithSensitiveField("api_key", "secret-key-123")
			sensitiveLogger.InfoEvent().
				Str("component", "JSONEndToEndTest").
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Bool("safe_logs_enabled", true).
				Msg("Processing request with JSON logging and safe logs")
			
			// Log debug information (should appear due to LOG_LEVEL=debug)
			requestLogger.DebugEvent().
				Str("debug_info", "detailed debugging information").
				Msg("Debug information logged")
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("JSON logging test completed"))
		})

		handler := correlation.Middleware(testHandler)

		req := httptest.NewRequest("GET", "/json-test", nil)
		req.Header.Set("X-Correlation-ID", correlationID)
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("ConsoleLoggingEndToEnd", func(t *testing.T) {
		// Test complete flow with console logging
		originalFormat := os.Getenv("LOG_FORMAT")
		originalLevel := os.Getenv("LOG_LEVEL")
		originalSafeLogs := os.Getenv("SAFE_LOGS")
		
		os.Setenv("LOG_FORMAT", "console")
		os.Setenv("LOG_LEVEL", "warn")
		os.Setenv("SAFE_LOGS", "false")
		
		defer func() {
			if originalFormat == "" {
				os.Unsetenv("LOG_FORMAT")
			} else {
				os.Setenv("LOG_FORMAT", originalFormat)
			}
			if originalLevel == "" {
				os.Unsetenv("LOG_LEVEL")
			} else {
				os.Setenv("LOG_LEVEL", originalLevel)
			}
			if originalSafeLogs == "" {
				os.Unsetenv("SAFE_LOGS")
			} else {
				os.Setenv("SAFE_LOGS", originalSafeLogs)
			}
		}()

		correlationID := "env-console-test-" + fmt.Sprintf("%d", time.Now().Unix())
		
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := logger.Default.WithCorrelationID(r.Context())
			
			// Log with URL that should NOT be redacted due to SAFE_LOGS=false
			requestLogger.Log("Processing request from http://normal-domain.com/api")
			
			// Debug log (should NOT appear due to LOG_LEVEL=warn)
			requestLogger.DebugEvent().
				Str("debug_info", "this should not appear").
				Msg("Debug information that should be filtered")
			
			// Warn log (should appear due to LOG_LEVEL=warn)
			requestLogger.WarnEvent().
				Str("component", "ConsoleEndToEndTest").
				Str("warning_type", "test_warning").
				Bool("safe_logs_enabled", false).
				Msg("Warning message in console format")
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Console logging test completed"))
		})

		handler := correlation.Middleware(testHandler)

		req := httptest.NewRequest("GET", "/console-test", nil)
		req.Header.Set("X-Correlation-ID", correlationID)
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
	})
}

// TestConcurrentEndToEndRequests tests concurrent request handling with correlation IDs
func TestConcurrentEndToEndRequests(t *testing.T) {
	t.Run("ConcurrentRequestsWithUniqueCorrelationIDs", func(t *testing.T) {
		correlationPrefix := "concurrent-test-" + fmt.Sprintf("%d", time.Now().Unix())
		
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := logger.Default.WithCorrelationID(r.Context())
			
			// Get the correlation ID from context
			correlationID, _ := correlation.CorrelationIDFromRequest(r)
			
			requestLogger.InfoEvent().
				Str("component", "ConcurrentHandler").
				Str("correlation_id", correlationID).
				Str("goroutine", fmt.Sprintf("%p", r)).
				Msg("Processing concurrent request")
			
			// Simulate some processing time
			processingTime := 50 * time.Millisecond
			time.Sleep(processingTime)
			
			requestLogger.InfoEvent().
				Str("component", "ConcurrentHandler").
				Str("correlation_id", correlationID).
				Dur("processing_time", processingTime).
				Msg("Concurrent request completed")
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("Processed: %s", correlationID)))
		})

		handler := correlation.Middleware(testHandler)

		// Test concurrent requests
		numRequests := 5
		done := make(chan bool, numRequests)
		results := make([]string, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(requestID int) {
				defer func() { done <- true }()

				correlationID := fmt.Sprintf("%s-%d", correlationPrefix, requestID)
				req := httptest.NewRequest("GET", fmt.Sprintf("/concurrent-test-%d", requestID), nil)
				req.Header.Set("X-Correlation-ID", correlationID)
				
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Request %d: Expected status %d, got %d", requestID, http.StatusOK, w.Code)
				}

				// Verify correlation ID was preserved
				if w.Header().Get("X-Correlation-ID") != correlationID {
					t.Errorf("Request %d: Correlation ID not preserved", requestID)
				}

				results[requestID] = w.Body.String()
			}(i)
		}

		// Wait for all requests to complete
		for i := 0; i < numRequests; i++ {
			<-done
		}

		// Verify all requests were processed with unique correlation IDs
		for i, result := range results {
			expectedCorrelationID := fmt.Sprintf("%s-%d", correlationPrefix, i)
			expectedResult := fmt.Sprintf("Processed: %s", expectedCorrelationID)
			if expectedResult != result {
				t.Errorf("Request %d: Expected result '%s', got '%s'", i, expectedResult, result)
			}
		}
	})
}

// TestFieldStandardizationEndToEnd tests field standardization across complete request flows
func TestFieldStandardizationEndToEnd(t *testing.T) {
	t.Run("StandardizedFieldsAcrossComponents", func(t *testing.T) {
		correlationID := "field-standardization-" + fmt.Sprintf("%d", time.Now().Unix())
		
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			requestLogger := logger.Default.WithCorrelationID(r.Context())
			
			// Use standardized field names consistently
			standardFields := map[string]interface{}{
				"component":       "StandardizationTestHandler",
				"method":          r.Method,
				"url":            r.URL.String(),
				"path":           r.URL.Path,
				"client_ip":      r.RemoteAddr,
				"user_agent":     r.UserAgent(),
				"content_length": r.ContentLength,
				"host":           r.Host,
			}

			contextualLogger := requestLogger.WithFields(standardFields)
			
			contextualLogger.InfoEvent().
				Msg("Request processing started with standardized fields")
			
			// Simulate different phases with consistent field usage
			phases := []string{"validation", "processing", "response"}
			
			for i, phase := range phases {
				phaseLogger := contextualLogger.WithFields(map[string]interface{}{
					"phase":        phase,
					"step":         i + 1,
					"phase_start":  time.Now(),
				})
				
				phaseLogger.DebugEvent().
					Msg(fmt.Sprintf("Starting %s phase", phase))
				
				// Simulate phase processing
				time.Sleep(10 * time.Millisecond)
				
				phaseLogger.InfoEvent().
					Dur("phase_duration", 10*time.Millisecond).
					Msg(fmt.Sprintf("Completed %s phase", phase))
			}
			
			// Final log with comprehensive standardized fields
			contextualLogger.InfoEvent().
				Dur("total_duration", time.Since(startTime)).
				Int("status_code", http.StatusOK).
				Time("completion_time", time.Now()).
				Int("phases_completed", len(phases)).
				Bool("success", true).
				Msg("Request processing completed with standardized fields")
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Standardized fields test completed"))
		})

		handler := correlation.Middleware(testHandler)

		req := httptest.NewRequest("GET", "/standardization-test?param=value", nil)
		req.Header.Set("X-Correlation-ID", correlationID)
		req.Header.Set("User-Agent", "StandardizationTest/1.0")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		if w.Header().Get("X-Correlation-ID") != correlationID {
			t.Error("Correlation ID not preserved in standardization test")
		}
	})
}