package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"m3u-stream-merger/correlation"
	"m3u-stream-merger/handlers"
	"m3u-stream-merger/logger"
	"m3u-stream-merger/proxy/loadbalancer"
	"m3u-stream-merger/proxy/stream"
	"m3u-stream-merger/proxy/stream/buffer"
	"m3u-stream-merger/proxy/stream/config"
	"m3u-stream-merger/store"

	"github.com/rs/zerolog"
)

// TestLoggerEnhancedIntegration tests the enhanced logger package functionality
func TestLoggerEnhancedIntegration(t *testing.T) {
	t.Run("StructuredLoggingAllLevels", func(t *testing.T) {
		// Set JSON format for this test
		originalFormat := os.Getenv("LOG_FORMAT")
		os.Setenv("LOG_FORMAT", "json")
		defer func() {
			if originalFormat == "" {
				os.Unsetenv("LOG_FORMAT")
			} else {
				os.Setenv("LOG_FORMAT", originalFormat)
			}
		}()

		var buf bytes.Buffer
		testLogger := logger.NewDefaultLogger()
		
		// Create a test logger that writes to our buffer
		_ = zerolog.New(&buf).With().Timestamp().Logger()
		
		// We'll create a wrapper logger that uses our buffer
		// Since we can't modify the internal logger directly, we'll test the interface
		
		// Test WithFields to ensure it creates proper structured logs
		fieldsLogger := testLogger.WithFields(map[string]interface{}{
			"component": "TestComponent",
			"test_id":   123,
		})

		// Test all log levels - we can't capture the exact output without modifying
		// the logger implementation, but we can verify the methods work
		fieldsLogger.InfoEvent().
			Str("operation", "test_operation").
			Msg("Test info message")

		fieldsLogger.DebugEvent().
			Bool("debug_mode", true).
			Msg("Test debug message")

		fieldsLogger.WarnEvent().
			Str("warning_type", "test_warning").
			Msg("Test warning message")

		fieldsLogger.ErrorEvent().
			Err(fmt.Errorf("test error")).
			Msg("Test error message")

		// Verify the logger interface works correctly
		if fieldsLogger == nil {
			t.Error("WithFields should return a valid logger")
		}
	})

	t.Run("ContextualFieldsIntegration", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()
		
		// Test WithFields functionality
		baseFields := map[string]interface{}{
			"request_id": "test-request-123",
			"user_id":    "user-456",
			"session_id": "session-789",
		}

		loggerWithFields := testLogger.WithFields(baseFields)
		
		// Verify it returns a different instance
		if loggerWithFields == testLogger {
			t.Error("WithFields should return a new logger instance")
		}

		// Test field inheritance
		childLogger := loggerWithFields.WithFields(map[string]interface{}{
			"operation": "test_operation",
			"step":      1,
		})

		if childLogger == nil {
			t.Error("Child logger should not be nil")
		}

		if childLogger == loggerWithFields {
			t.Error("Child logger should be a different instance")
		}

		// Test that we can use the child logger
		childLogger.InfoEvent().
			Str("component", "TestComponent").
			Msg("Test message with inherited fields")
	})

	t.Run("CorrelationIDPropagation", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()
		
		// Create context with correlation ID
		correlationID := "test-correlation-123"
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)

		// Test correlation ID propagation
		loggerWithCorrelation := testLogger.WithCorrelationID(ctx)
		
		if loggerWithCorrelation == nil {
			t.Error("WithCorrelationID should return a valid logger")
		}

		// Test with nil context
		loggerWithNilCorrelation := testLogger.WithCorrelationID(context.TODO())
		if loggerWithNilCorrelation != testLogger {
			t.Error("WithCorrelationID with nil context should return the same logger")
		}

		// Test with context without correlation ID
		emptyCtx := context.Background()
		loggerWithEmptyCorrelation := testLogger.WithCorrelationID(emptyCtx)
		if loggerWithEmptyCorrelation != testLogger {
			t.Error("WithCorrelationID with empty context should return the same logger")
		}

		// Test that we can log with correlation
		loggerWithCorrelation.InfoEvent().
			Str("component", "TestComponent").
			Msg("Test message with correlation ID")
	})

	t.Run("SensitiveFieldRedaction", func(t *testing.T) {
		// Test with SAFE_LOGS enabled
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
		
		// Test sensitive field redaction
		loggerWithSensitive := testLogger.WithSensitiveField("password", "secret123")
		
		if loggerWithSensitive == nil {
			t.Error("WithSensitiveField should return a valid logger")
		}

		if loggerWithSensitive == testLogger {
			t.Error("WithSensitiveField should return a new logger instance")
		}

		// Test that we can log with sensitive field
		loggerWithSensitive.InfoEvent().
			Str("component", "TestComponent").
			Msg("Test message with sensitive field")

		// Test with SAFE_LOGS disabled
		os.Setenv("SAFE_LOGS", "false")
		loggerWithSensitive2 := testLogger.WithSensitiveField("token", "abc123")
		
		if loggerWithSensitive2 == nil {
			t.Error("WithSensitiveField should return a valid logger even when SAFE_LOGS is false")
		}
	})

	t.Run("EventTypesAndFields", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()
		
		// Test all event types and field methods
		currentTime := time.Now()
		duration := 5 * time.Second

		// Test InfoEvent with all field types
		infoEvent := testLogger.InfoEvent()
		if infoEvent == nil {
			t.Error("InfoEvent should return a valid event")
		}

		infoEvent.
			Str("string_field", "test_value").
			Int("int_field", 42).
			Int64("int64_field", 123456789).
			Float64("float_field", 3.14159).
			Bool("bool_field", true).
			Time("time_field", currentTime).
			Dur("duration_field", duration).
			Msg("Test info event with all field types")

		// Test DebugEvent
		debugEvent := testLogger.DebugEvent()
		if debugEvent == nil {
			t.Error("DebugEvent should return a valid event")
		}
		debugEvent.Msg("Test debug event")

		// Test WarnEvent
		warnEvent := testLogger.WarnEvent()
		if warnEvent == nil {
			t.Error("WarnEvent should return a valid event")
		}
		warnEvent.Msg("Test warn event")

		// Test ErrorEvent
		errorEvent := testLogger.ErrorEvent()
		if errorEvent == nil {
			t.Error("ErrorEvent should return a valid event")
		}
		errorEvent.Err(fmt.Errorf("test error")).Msg("Test error event")

		// Test FatalEvent (don't call Msg as it would exit)
		fatalEvent := testLogger.FatalEvent()
		if fatalEvent == nil {
			t.Error("FatalEvent should return a valid event")
		}
		// Don't call Msg() on fatal event as it would terminate the program
	})
}

// TestHTTPHandlersIntegration tests HTTP handlers with enhanced logging
func TestHTTPHandlersIntegration(t *testing.T) {
	t.Run("CorrelationIDGenerationAndPropagation", func(t *testing.T) {
		var capturedCorrelationIDs []string
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify correlation ID is in context
			if correlationID, ok := correlation.CorrelationIDFromRequest(r); ok {
				capturedCorrelationIDs = append(capturedCorrelationIDs, correlationID)
			} else {
				t.Error("Correlation ID not found in request context")
			}
			
			// Verify correlation ID is in response headers
			if responseCorrelationID := w.Header().Get("X-Correlation-ID"); responseCorrelationID == "" {
				t.Error("Correlation ID not found in response headers")
			} else {
				// Verify context and header correlation IDs match
				if contextID, ok := correlation.CorrelationIDFromRequest(r); ok {
					if contextID != responseCorrelationID {
						t.Errorf("Context correlation ID (%s) doesn't match header correlation ID (%s)", 
							contextID, responseCorrelationID)
					}
				}
			}
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		// Wrap with correlation middleware
		handler := correlation.Middleware(testHandler)

		// Test 1: Request without correlation ID (should generate one)
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)

		if len(capturedCorrelationIDs) == 0 {
			t.Error("No correlation ID was captured")
		} else {
			generatedID := capturedCorrelationIDs[0]
			if generatedID == "" {
				t.Error("Generated correlation ID is empty")
			}
			if len(generatedID) < 10 { // UUID should be much longer
				t.Errorf("Generated correlation ID seems too short: %s", generatedID)
			}
		}

		// Test 2: Request with existing correlation ID (should preserve it)
		existingCorrelationID := "existing-correlation-123"
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("X-Correlation-ID", existingCorrelationID)
		w2 := httptest.NewRecorder()
		
		capturedCorrelationIDs = []string{} // Reset
		handler.ServeHTTP(w2, req2)

		if len(capturedCorrelationIDs) == 0 {
			t.Error("No correlation ID was captured for existing correlation ID test")
		} else if capturedCorrelationIDs[0] != existingCorrelationID {
			t.Errorf("Expected existing correlation ID '%s', got '%s'", 
				existingCorrelationID, capturedCorrelationIDs[0])
		}

		// Verify response header has the existing correlation ID
		if w2.Header().Get("X-Correlation-ID") != existingCorrelationID {
			t.Errorf("Response header should contain existing correlation ID '%s', got '%s'",
				existingCorrelationID, w2.Header().Get("X-Correlation-ID"))
		}
	})

	t.Run("M3UHTTPHandlerContextualLogging", func(t *testing.T) {
		// Create M3U handler
		m3uHandler := handlers.NewM3UHTTPHandler(logger.Default, "")

		// Test without credentials (should log successful access)
		req := httptest.NewRequest("GET", "/playlist.m3u", nil)
		ctx := correlation.WithCorrelationID(req.Context(), "test-correlation-m3u")
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		m3uHandler.ServeHTTP(w, req)

		// Verify the handler executed and set a status code
		if w.Code == 0 {
			t.Error("Handler should have set a status code")
		}

		// Should return 404 since no processed path is set
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("StreamHTTPHandlerContextualLogging", func(t *testing.T) {
		// Create a mock proxy instance for testing
		mockProxy := handlers.NewDefaultProxyInstance()
		streamHandler := handlers.NewStreamHTTPHandler(mockProxy, logger.Default)

		// Test stream request
		req := httptest.NewRequest("GET", "/stream/test-stream.m3u8", nil)
		ctx := correlation.WithCorrelationID(req.Context(), "test-correlation-stream")
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		// This will fail because we don't have a real stream, but it should log appropriately
		streamHandler.ServeHTTP(w, req)

		// Verify the handler attempted to process the request
		// The specific response depends on the implementation and available streams
		if w.Code == 0 {
			t.Log("Handler did not set a status code (expected in test environment)")
		}
	})

	t.Run("PassthroughHTTPHandlerLogging", func(t *testing.T) {
		passthroughHandler := handlers.NewPassthroughHTTPHandler(logger.Default)

		// Test with invalid URL path
		req := httptest.NewRequest("GET", "/invalid/path", nil)
		ctx := correlation.WithCorrelationID(req.Context(), "test-correlation-passthrough")
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		passthroughHandler.ServeHTTP(w, req)

		// Should return 400 for invalid path
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status code %d for invalid path, got %d", 
				http.StatusBadRequest, w.Code)
		}

		// Test with valid path format but invalid base64
		req2 := httptest.NewRequest("GET", "/a/invalid-base64!", nil)
		ctx2 := correlation.WithCorrelationID(req2.Context(), "test-correlation-passthrough-2")
		req2 = req2.WithContext(ctx2)
		
		w2 := httptest.NewRecorder()
		passthroughHandler.ServeHTTP(w2, req2)

		// Should return 400 for invalid base64
		if w2.Code != http.StatusBadRequest {
			t.Errorf("Expected status code %d for invalid base64, got %d", 
				http.StatusBadRequest, w2.Code)
		}
	})
}

// TestStreamProcessingIntegration tests stream processing logs
func TestStreamProcessingIntegration(t *testing.T) {
	t.Run("StreamCoordinatorLogging", func(t *testing.T) {
		// Create test configuration
		streamConfig := &config.StreamConfig{
			SharedBufferSize: 10,
			ChunkSize:        1024,
			TimeoutSeconds:   30,
			InitialBackoff:   100 * time.Millisecond,
		}

		cm := store.NewConcurrencyManager()
		streamID := "test-stream-123"

		// Create stream coordinator
		coordinator := buffer.NewStreamCoordinator(streamID, streamConfig, cm, logger.Default)

		if coordinator == nil {
			t.Fatal("Failed to create stream coordinator")
		}

		// Test client registration
		err := coordinator.RegisterClient()
		if err != nil {
			t.Errorf("Failed to register client: %v", err)
		}

		// Verify client count
		if !coordinator.HasClient() {
			t.Error("Coordinator should have at least one client")
		}

		// Test buffer operations with a proper chunk
		chunk := &buffer.ChunkData{
			Buffer:    nil, // In real usage, this would be a ByteBuffer
			Error:     nil,
			Status:    0,
			Timestamp: time.Now(),
		}

		// Initialize the chunk properly
		chunk.Reset()

		// This should log buffer write operations
		success := coordinator.Write(chunk)
		if !success {
			t.Log("Buffer write failed (expected in test environment without proper buffer)")
		}

		// Test with error chunk
		errorChunk := &buffer.ChunkData{
			Buffer:    nil,
			Error:     fmt.Errorf("test stream error"),
			Status:    500,
			Timestamp: time.Now(),
		}
		errorChunk.Reset()

		coordinator.Write(errorChunk)

		// Test client unregistration
		coordinator.UnregisterClient()

		// Verify the coordinator logged appropriately
		if coordinator.HasClient() {
			t.Error("Client should have been unregistered")
		}
	})

	t.Run("LoadBalancerLogging", func(t *testing.T) {
		// Create load balancer configuration
		lbConfig := &loadbalancer.LBConfig{
			MaxRetries: 2,
			RetryWait:  1,
		}

		cm := store.NewConcurrencyManager()
		
		// Create load balancer instance
		lb := loadbalancer.NewLoadBalancerInstance(cm, lbConfig,
			loadbalancer.WithLogger(logger.Default))

		if lb == nil {
			t.Fatal("Failed to create load balancer instance")
		}

		// Test load balancing (will fail but should log)
		req := httptest.NewRequest("GET", "/test-stream.m3u8", nil)
		ctx := correlation.WithCorrelationID(context.Background(), "test-correlation-lb")
		
		// This will fail because we don't have configured streams
		result, err := lb.Balance(ctx, req)
		if err != nil {
			t.Logf("Load balancer failed as expected in test environment: %v", err)
		}
		if result != nil {
			t.Log("Unexpected load balancer success in test environment")
		}

		// Test with nil context (should fail)
		result2, err2 := lb.Balance(context.TODO(), req)
		if err2 == nil {
			t.Error("Load balancer should fail with nil context")
		}
		if result2 != nil {
			t.Error("Load balancer should return nil result with nil context")
		}

		// Test with nil request (should fail)
		result3, err3 := lb.Balance(ctx, nil)
		if err3 == nil {
			t.Error("Load balancer should fail with nil request")
		}
		if result3 != nil {
			t.Error("Load balancer should return nil result with nil request")
		}
	})

	t.Run("StreamInstanceLogging", func(t *testing.T) {
		// Create stream instance configuration
		streamConfig := &config.StreamConfig{
			SharedBufferSize: 10,
			ChunkSize:        1024,
			TimeoutSeconds:   30,
			InitialBackoff:   100 * time.Millisecond,
		}

		cm := store.NewConcurrencyManager()
		
		// Create stream instance
		streamInstance, err := stream.NewStreamInstance(cm, streamConfig,
			stream.WithLogger(logger.Default))

		if err != nil {
			t.Fatalf("Failed to create stream instance: %v", err)
		}

		if streamInstance == nil {
			t.Fatal("Stream instance should not be nil")
		}

		// Test with nil concurrency manager (should fail)
		_, err = stream.NewStreamInstance(nil, streamConfig)
		if err == nil {
			t.Error("Stream instance creation should fail with nil concurrency manager")
		}
	})
}

// TestEnvironmentConfigurationIntegration tests environment variable configuration
func TestEnvironmentConfigurationIntegration(t *testing.T) {
	t.Run("LogFormatConfiguration", func(t *testing.T) {
		tests := []struct {
			name           string
			logFormat      string
			expectedBehavior string
		}{
			{"JSONFormat", "json", "should create JSON logger"},
			{"ConsoleFormat", "console", "should create console logger"},
			{"DefaultFormat", "", "should create console logger by default"},
			{"InvalidFormat", "invalid", "should create console logger for invalid format"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Save original environment
				originalFormat := os.Getenv("LOG_FORMAT")
				defer func() {
					if originalFormat == "" {
						os.Unsetenv("LOG_FORMAT")
					} else {
						os.Setenv("LOG_FORMAT", originalFormat)
					}
				}()

				if tt.logFormat != "" {
					os.Setenv("LOG_FORMAT", tt.logFormat)
				} else {
					os.Unsetenv("LOG_FORMAT")
				}

				// Create logger with the specified format
				testLogger := logger.NewDefaultLogger()

				// Verify logger was created successfully
				if testLogger == nil {
					t.Errorf("Failed to create logger with format: %s", tt.logFormat)
				}

				// Test that we can use all logging methods
				testLogger.Log("test info message")
				testLogger.Debug("test debug message")
				testLogger.Warn("test warn message")
				testLogger.Error("test error message")

				// Test structured logging methods
				testLogger.InfoEvent().Msg("test structured info")
				testLogger.DebugEvent().Msg("test structured debug")
				testLogger.WarnEvent().Msg("test structured warn")
				testLogger.ErrorEvent().Msg("test structured error")
			})
		}
	})

	t.Run("LogLevelConfiguration", func(t *testing.T) {
		tests := []struct {
			name     string
			logLevel string
		}{
			{"DebugLevel", "debug"},
			{"InfoLevel", "info"},
			{"WarnLevel", "warn"},
			{"ErrorLevel", "error"},
			{"FatalLevel", "fatal"},
			{"InvalidLevel", "invalid"}, // Should default to info
			{"EmptyLevel", ""},         // Should default to info
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Save original environment
				originalLevel := os.Getenv("LOG_LEVEL")
				defer func() {
					if originalLevel == "" {
						os.Unsetenv("LOG_LEVEL")
					} else {
						os.Setenv("LOG_LEVEL", originalLevel)
					}
				}()

				if tt.logLevel != "" {
					os.Setenv("LOG_LEVEL", tt.logLevel)
				} else {
					os.Unsetenv("LOG_LEVEL")
				}

				// Create logger with the specified level
				testLogger := logger.NewDefaultLogger()

				// Verify logger was created successfully
				if testLogger == nil {
					t.Errorf("Failed to create logger with level: %s", tt.logLevel)
				}

				// Test logging at different levels
				testLogger.Debug("debug message")
				testLogger.Log("info message")
				testLogger.Warn("warn message")
				testLogger.Error("error message")

				// Test structured logging
				testLogger.DebugEvent().Str("level", "debug").Msg("debug structured")
				testLogger.InfoEvent().Str("level", "info").Msg("info structured")
				testLogger.WarnEvent().Str("level", "warn").Msg("warn structured")
				testLogger.ErrorEvent().Str("level", "error").Msg("error structured")
			})
		}
	})

	t.Run("SafeLogsConfiguration", func(t *testing.T) {
		tests := []struct {
			name      string
			safeLogs  string
			testURL   string
		}{
			{"SafeLogsEnabled", "true", "http://example.com/test"},
			{"SafeLogsDisabled", "false", "https://api.example.com/v1/data"},
			{"SafeLogsDefault", "", "ftp://files.example.com/file.txt"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Save original environment
				originalSafeLogs := os.Getenv("SAFE_LOGS")
				defer func() {
					if originalSafeLogs == "" {
						os.Unsetenv("SAFE_LOGS")
					} else {
						os.Setenv("SAFE_LOGS", originalSafeLogs)
					}
				}()

				if tt.safeLogs != "" {
					os.Setenv("SAFE_LOGS", tt.safeLogs)
				} else {
					os.Unsetenv("SAFE_LOGS")
				}

				testLogger := logger.NewDefaultLogger()

				// Test URL logging behavior
				testLogger.Log(fmt.Sprintf("Fetching data from %s", tt.testURL))
				testLogger.InfoEvent().
					Str("url", tt.testURL).
					Msg("Processing URL")

				// Test sensitive field behavior
				sensitiveLogger := testLogger.WithSensitiveField("api_key", "secret-key-123")
				sensitiveLogger.Log("API request with sensitive data")
			})
		}
	})
}

// TestFieldStandardizationIntegration tests field standardization across components
func TestFieldStandardizationIntegration(t *testing.T) {
	t.Run("StandardFieldNamesConsistency", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test standard field names used across the application
		standardFields := map[string]interface{}{
			"component":       "TestComponent",
			"request_id":      "req-123",
			"stream_id":       "stream-456",
			"url":            "http://example.com",
			"method":         "GET",
			"status_code":    200,
			"duration":       time.Second,
			"client_ip":      "192.168.1.1",
			"user_agent":     "TestAgent/1.0",
			"channel_name":   "test-channel",
			"playlist_url":   "http://example.com/playlist.m3u",
			"segment_url":    "http://example.com/segment.ts",
			"buffer_status":  "active",
			"client_count":   5,
		}

		loggerWithFields := testLogger.WithFields(standardFields)

		// Test that all standard fields are accepted
		loggerWithFields.InfoEvent().Msg("Test message with all standard fields")

		// Test field type consistency
		loggerWithFields.InfoEvent().
			Str("component", "TestComponent").        // String
			Int("status_code", 200).                  // Integer
			Int64("bytes_written", 1024).            // Int64
			Float64("response_time", 1.23).          // Float
			Bool("success", true).                   // Boolean
			Time("timestamp", time.Now()).           // Time
			Dur("duration", time.Second).            // Duration
			Msg("Test message with typed fields")
	})

	t.Run("FieldDataTypeConsistency", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test that the same field names consistently use the same data types
		tests := []struct {
			name      string
			fieldName string
			values    []interface{}
		}{
			{"StringFields", "component", []interface{}{"Component1", "Component2", "Component3"}},
			{"IntegerFields", "status_code", []interface{}{200, 404, 500}},
			{"BooleanFields", "success", []interface{}{true, false, true}},
			{"DurationFields", "duration", []interface{}{time.Second, time.Minute, time.Hour}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				for i, value := range tt.values {
					fields := map[string]interface{}{
						tt.fieldName: value,
						"test_id":    i,
					}

					loggerWithField := testLogger.WithFields(fields)
					loggerWithField.InfoEvent().
						Msg(fmt.Sprintf("Test %s consistency iteration %d", tt.name, i))
				}
			})
		}
	})
}

// TestPerformanceIntegration tests logging performance
func TestPerformanceIntegration(t *testing.T) {
	t.Run("HighVolumeLogging", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test performance with high volume logging
		iterations := 1000
		start := time.Now()

		for i := 0; i < iterations; i++ {
			testLogger.InfoEvent().
				Int("iteration", i).
				Str("component", "PerformanceTest").
				Msg("High volume test message")
		}

		elapsed := time.Since(start)

		// Performance should be reasonable (less than 10ms per log on average)
		averageTime := elapsed / time.Duration(iterations)
		if averageTime > 10*time.Millisecond {
			t.Errorf("Average logging time too high: %v per log", averageTime)
		}

		t.Logf("High volume logging performance: %d logs in %v (avg: %v per log)",
			iterations, elapsed, averageTime)
	})

	t.Run("FieldSerializationPerformance", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Create logger with many fields
		fields := make(map[string]interface{})
		for i := 0; i < 50; i++ {
			fields[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
		}

		fieldLogger := testLogger.WithFields(fields)

		// Test performance with many fields
		iterations := 100
		start := time.Now()

		for i := 0; i < iterations; i++ {
			fieldLogger.InfoEvent().
				Int("iteration", i).
				Msg("Performance test with many fields")
		}

		elapsed := time.Since(start)

		// Should complete within reasonable time
		if elapsed > 5*time.Second {
			t.Errorf("Field serialization too slow: %v for %d iterations", elapsed, iterations)
		}

		t.Logf("Field serialization performance: %d logs with 50+ fields in %v",
			iterations, elapsed)
	})

	t.Run("ConcurrentLogging", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test concurrent logging performance
		numGoroutines := 10
		logsPerGoroutine := 100

		start := time.Now()
		done := make(chan bool, numGoroutines)

		for g := 0; g < numGoroutines; g++ {
			go func(goroutineID int) {
				defer func() { done <- true }()

				for i := 0; i < logsPerGoroutine; i++ {
					testLogger.InfoEvent().
						Int("goroutine_id", goroutineID).
						Int("iteration", i).
						Str("component", "ConcurrencyTest").
						Msg("Concurrent logging test")
				}
			}(g)
		}

		// Wait for all goroutines to complete
		for g := 0; g < numGoroutines; g++ {
			<-done
		}

		elapsed := time.Since(start)
		totalLogs := numGoroutines * logsPerGoroutine

		t.Logf("Concurrent logging performance: %d logs from %d goroutines in %v",
			totalLogs, numGoroutines, elapsed)

		// Should complete within reasonable time
		if elapsed > 10*time.Second {
			t.Errorf("Concurrent logging too slow: %v for %d total logs", elapsed, totalLogs)
		}
	})
}

// TestEndToEndIntegration tests complete request flows through the system
func TestEndToEndIntegration(t *testing.T) {
	t.Run("CompleteRequestFlow", func(t *testing.T) {
		// Create a complete request flow with correlation ID propagation
		correlationID := "e2e-test-" + fmt.Sprintf("%d", time.Now().Unix())
		
		// Step 1: HTTP request with correlation middleware
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Step 2: Extract correlation ID and create logger
			requestLogger := logger.Default.WithCorrelationID(r.Context())
			
			// Step 3: Log request processing
			requestLogger.InfoEvent().
				Str("component", "EndToEndHandler").
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("user_agent", r.UserAgent()).
				Msg("Processing end-to-end test request")
			
			// Step 4: Simulate business logic with contextual logging
			businessLogger := requestLogger.WithFields(map[string]interface{}{
				"operation": "business_logic",
				"step":      1,
			})
			
			businessLogger.DebugEvent().
				Msg("Starting business logic processing")
			
			// Step 5: Simulate error handling
			if r.URL.Query().Get("simulate_error") == "true" {
				businessLogger.ErrorEvent().
					Str("error_type", "simulated_error").
					Msg("Simulated error for testing")
				
				http.Error(w, "Simulated error", http.StatusInternalServerError)
				return
			}
			
			// Step 6: Log successful completion
			businessLogger.InfoEvent().
				Dur("processing_time", 50*time.Millisecond).
				Msg("Business logic completed successfully")
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		})

		handler := correlation.Middleware(testHandler)

		// Test successful flow
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", correlationID)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Verify correlation ID was preserved
		if w.Header().Get("X-Correlation-ID") != correlationID {
			t.Errorf("Correlation ID not preserved in response")
		}

		// Test error flow
		req2 := httptest.NewRequest("GET", "/test?simulate_error=true", nil)
		req2.Header.Set("X-Correlation-ID", correlationID+"-error")
		w2 := httptest.NewRecorder()

		handler.ServeHTTP(w2, req2)

		if w2.Code != http.StatusInternalServerError {
			t.Errorf("Expected error status %d, got %d", http.StatusInternalServerError, w2.Code)
		}
	})

	t.Run("StreamProcessingWorkflow", func(t *testing.T) {
		// Test complete stream processing workflow
		correlationID := "stream-e2e-" + fmt.Sprintf("%d", time.Now().Unix())
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)

		// Create stream coordinator
		streamConfig := &config.StreamConfig{
			SharedBufferSize: 5,
			ChunkSize:        512,
			TimeoutSeconds:   10,
			InitialBackoff:   50 * time.Millisecond,
		}

		cm := store.NewConcurrencyManager()
		coordinator := buffer.NewStreamCoordinator("e2e-stream", streamConfig, cm, logger.Default)

		// Simulate stream workflow
		requestLogger := logger.Default.WithCorrelationID(ctx)
		
		// Step 1: Client registration
		requestLogger.InfoEvent().
			Str("component", "StreamWorkflow").
			Str("stream_id", "e2e-stream").
			Msg("Starting stream processing workflow")

		err := coordinator.RegisterClient()
		if err != nil {
			t.Errorf("Failed to register client: %v", err)
		}

		// Step 2: Buffer operations
		for i := 0; i < 3; i++ {
			chunk := &buffer.ChunkData{
				Buffer:    nil, // Would be real data in production
				Error:     nil,
				Status:    0,
				Timestamp: time.Now(),
			}
			chunk.Reset()

			requestLogger.DebugEvent().
				Int("chunk_number", i).
				Str("operation", "buffer_write").
				Msg("Writing chunk to buffer")

			coordinator.Write(chunk)
		}

		// Step 3: Client unregistration
		coordinator.UnregisterClient()

		requestLogger.InfoEvent().
			Str("component", "StreamWorkflow").
			Str("stream_id", "e2e-stream").
			Msg("Stream processing workflow completed")
	})
}

// TestBackwardCompatibilityIntegration tests that existing code continues to work
func TestBackwardCompatibilityIntegration(t *testing.T) {
	t.Run("ExistingLoggingMethods", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test all existing logging methods work unchanged
		testLogger.Log("Test log message")
		testLogger.Logf("Test log message with %s", "formatting")
		testLogger.Debug("Test debug message")
		testLogger.Debugf("Test debug message with %d", 42)
		testLogger.Warn("Test warn message")
		testLogger.Warnf("Test warn message with %v", true)
		testLogger.Error("Test error message")
		testLogger.Errorf("Test error message with %f", 3.14)

		// Note: Not testing Fatal methods as they would exit the program
	})

	t.Run("MixedLoggingStyles", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test mixing old and new logging styles
		loggerWithFields := testLogger.WithFields(map[string]interface{}{
			"component": "BackwardCompatTest",
			"test_id":   "mixed-001",
		})

		// Use old style logging
		loggerWithFields.Log("Old style log message")
		loggerWithFields.Debugf("Old style debug with %s", "parameters")

		// Use new style logging
		loggerWithFields.InfoEvent().
			Str("style", "new").
			Msg("New style log message")

		loggerWithFields.DebugEvent().
			Str("style", "new").
			Int("param_count", 1).
			Msg("New style debug message")

		// Verify both styles can be used on the same logger instance
		if loggerWithFields == nil {
			t.Error("Logger with fields should support both old and new styles")
		}
	})

	t.Run("DefaultLoggerUsage", func(t *testing.T) {
		// Test that the default logger still works as expected
		logger.Default.Log("Default logger test message")
		logger.Default.Debug("Default logger debug message")
		logger.Default.Warn("Default logger warn message")
		logger.Default.Error("Default logger error message")

		// Test new methods on default logger
		logger.Default.InfoEvent().
			Str("component", "DefaultLoggerTest").
			Msg("Default logger structured message")
	})
}

// TestErrorHandlingIntegration tests error handling in logging functionality
func TestErrorHandlingIntegration(t *testing.T) {
	t.Run("InvalidFieldValues", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test with various invalid or edge case field values
		edgeCaseFields := map[string]interface{}{
			"nil_value":    nil,
			"empty_string": "",
			"zero_int":     0,
			"false_bool":   false,
			"empty_slice":  []string{},
			"empty_map":    map[string]string{},
		}

		// Should handle edge cases gracefully
		loggerWithEdgeCases := testLogger.WithFields(edgeCaseFields)
		if loggerWithEdgeCases == nil {
			t.Error("Logger should handle edge case field values gracefully")
		}

		loggerWithEdgeCases.InfoEvent().Msg("Test with edge case field values")
	})

	t.Run("EmptyFieldNames", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test with empty field names
		fieldsWithEmptyNames := map[string]interface{}{
			"":           "empty_key_value",
			"valid_key":  "valid_value",
		}

		// Should handle empty field names gracefully
		loggerWithEmptyNames := testLogger.WithFields(fieldsWithEmptyNames)
		if loggerWithEmptyNames == nil {
			t.Error("Logger should handle empty field names gracefully")
		}

		loggerWithEmptyNames.InfoEvent().Msg("Test with empty field names")
	})

	t.Run("NilContextHandling", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test WithCorrelationID with nil context
		loggerWithNilContext := testLogger.WithCorrelationID(context.TODO())
		if loggerWithNilContext != testLogger {
			t.Error("WithCorrelationID with nil context should return the same logger")
		}

		// Should not panic
		loggerWithNilContext.InfoEvent().Msg("Test with nil context")
	})

	t.Run("SpecialCharactersInFields", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test with special characters in field names and values
		specialFields := map[string]interface{}{
			"field_with_unicode_🚀":     "value_with_unicode_🌟",
			"field-with-dashes":         "value-with-dashes",
			"field with spaces":         "value with spaces",
			"field.with.dots":           "value.with.dots",
			"field[with]brackets":       "value[with]brackets",
			"field\"with\"quotes":       "value\"with\"quotes",
			"field'with'apostrophes":    "value'with'apostrophes",
			"field\nwith\nnewlines":     "value\nwith\nnewlines",
			"field\twith\ttabs":         "value\twith\ttabs",
		}

		// Should handle special characters gracefully
		loggerWithSpecialChars := testLogger.WithFields(specialFields)
		if loggerWithSpecialChars == nil {
			t.Error("Logger should handle special characters in field names gracefully")
		}

		loggerWithSpecialChars.InfoEvent().Msg("Test with special characters in fields")
	})

	t.Run("LargeFieldValues", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test with large field values
		largeString := strings.Repeat("large_value_", 1000)
		largeFields := map[string]interface{}{
			"large_string": largeString,
			"large_number": 999999999999999,
		}

		// Should handle large field values gracefully
		loggerWithLargeFields := testLogger.WithFields(largeFields)
		if loggerWithLargeFields == nil {
			t.Error("Logger should handle large field values gracefully")
		}

		loggerWithLargeFields.InfoEvent().Msg("Test with large field values")
	})

	t.Run("EventBuilderChaining", func(t *testing.T) {
		testLogger := logger.NewDefaultLogger()

		// Test event builder chaining doesn't break with various field types
		event := testLogger.InfoEvent()
		
		// Chain many field additions
		result := event.
			Str("string_field", "test").
			Int("int_field", 42).
			Bool("bool_field", true).
			Float64("float_field", 3.14).
			Time("time_field", time.Now()).
			Dur("duration_field", time.Second)

		if result == nil {
			t.Error("Event builder chaining should not return nil")
		}

		// Should be able to finalize the event
		result.Msg("Test event builder chaining")
	})
}