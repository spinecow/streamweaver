package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"m3u-stream-merger/correlation"
	"m3u-stream-merger/logger"
	"m3u-stream-merger/proxy/loadbalancer"
	"m3u-stream-merger/proxy/stream"
	"m3u-stream-merger/proxy/stream/buffer"
	"m3u-stream-merger/proxy/stream/config"
	"m3u-stream-merger/store"
)

// TestStreamProcessingLoggingIntegration tests stream processing with enhanced logging
func TestStreamProcessingLoggingIntegration(t *testing.T) {
	t.Run("StreamCoordinatorLifecycle", func(t *testing.T) {
		// Create test configuration
		streamConfig := &config.StreamConfig{
			SharedBufferSize: 5,
			ChunkSize:        1024,
			TimeoutSeconds:   10,
			InitialBackoff:   100 * time.Millisecond,
		}

		cm := store.NewConcurrencyManager()
		streamID := "integration-test-stream"

		// Create stream coordinator with correlation context
		correlationID := "stream-coordinator-test-123"
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)
		
		// Create logger with correlation ID
		testLogger := logger.Default.WithCorrelationID(ctx)
		
		// Create coordinator
		coordinator := buffer.NewStreamCoordinator(streamID, streamConfig, cm, testLogger)

		if coordinator == nil {
			t.Fatal("Failed to create stream coordinator")
		}

		// Test client registration with logging
		testLogger.InfoEvent().
			Str("component", "StreamCoordinatorTest").
			Str("operation", "client_registration").
			Str("stream_id", streamID).
			Msg("Testing client registration")

		err := coordinator.RegisterClient()
		if err != nil {
			t.Errorf("Failed to register client: %v", err)
		}

		if !coordinator.HasClient() {
			t.Error("Coordinator should have a client registered")
		}

		// Test buffer write operations with proper chunks
		testLogger.InfoEvent().
			Str("component", "StreamCoordinatorTest").
			Str("operation", "buffer_write").
			Str("stream_id", streamID).
			Msg("Testing buffer write operations")

		for i := 0; i < 3; i++ {
			chunk := &buffer.ChunkData{
				Buffer:    nil, // In real usage, this would be a ByteBuffer
				Error:     nil,
				Status:    0,
				Timestamp: time.Now(),
			}

			// Initialize the chunk properly
			chunk.Reset()

			testLogger.DebugEvent().
				Str("component", "StreamCoordinatorTest").
				Int("chunk_number", i).
				Str("stream_id", streamID).
				Msg("Writing test chunk to buffer")

			success := coordinator.Write(chunk)
			if !success {
				t.Logf("Buffer write %d failed (expected in test environment)", i)
			}
		}

		// Test error chunk handling
		testLogger.InfoEvent().
			Str("component", "StreamCoordinatorTest").
			Str("operation", "error_handling").
			Str("stream_id", streamID).
			Msg("Testing error chunk handling")

		errorChunk := &buffer.ChunkData{
			Buffer:    nil,
			Error:     fmt.Errorf("test stream error"),
			Status:    500,
			Timestamp: time.Now(),
		}
		errorChunk.Reset()

		coordinator.Write(errorChunk)

		// Test client unregistration
		testLogger.InfoEvent().
			Str("component", "StreamCoordinatorTest").
			Str("operation", "client_unregistration").
			Str("stream_id", streamID).
			Msg("Testing client unregistration")

		coordinator.UnregisterClient()

		if coordinator.HasClient() {
			t.Error("Coordinator should not have any clients after unregistration")
		}

		testLogger.InfoEvent().
			Str("component", "StreamCoordinatorTest").
			Str("stream_id", streamID).
			Msg("Stream coordinator lifecycle test completed")
	})

	t.Run("LoadBalancerIntegration", func(t *testing.T) {
		// Create load balancer configuration
		lbConfig := &loadbalancer.LBConfig{
			MaxRetries: 2,
			RetryWait:  1,
		}

		cm := store.NewConcurrencyManager()
		correlationID := "load-balancer-test-456"
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)
		
		testLogger := logger.Default.WithCorrelationID(ctx)

		// Create load balancer instance
		lb := loadbalancer.NewLoadBalancerInstance(cm, lbConfig,
			loadbalancer.WithLogger(testLogger))

		if lb == nil {
			t.Fatal("Failed to create load balancer instance")
		}

		testLogger.InfoEvent().
			Str("component", "LoadBalancerTest").
			Str("operation", "balance_request").
			Msg("Testing load balancer with contextual logging")

		// Test load balancing (will fail but should log appropriately)
		req := createTestHTTPRequest("GET", "/test-stream.m3u8")
		
		result, err := lb.Balance(ctx, req)
		if err != nil {
			testLogger.WarnEvent().
				Str("component", "LoadBalancerTest").
				Err(err).
				Msg("Load balancer failed as expected in test environment")
		}
		if result != nil {
			t.Log("Unexpected load balancer success in test environment")
		}

		// Test error scenarios with logging
		testLogger.InfoEvent().
			Str("component", "LoadBalancerTest").
			Str("operation", "error_scenarios").
			Msg("Testing load balancer error scenarios")

		// Test with nil context
		result2, err2 := lb.Balance(nil, req)
		if err2 == nil {
			t.Error("Load balancer should fail with nil context")
		} else {
			testLogger.DebugEvent().
				Str("component", "LoadBalancerTest").
				Err(err2).
				Msg("Load balancer correctly failed with nil context")
		}
		if result2 != nil {
			t.Error("Load balancer should return nil result with nil context")
		}

		// Test with nil request
		result3, err3 := lb.Balance(ctx, nil)
		if err3 == nil {
			t.Error("Load balancer should fail with nil request")
		} else {
			testLogger.DebugEvent().
				Str("component", "LoadBalancerTest").
				Err(err3).
				Msg("Load balancer correctly failed with nil request")
		}
		if result3 != nil {
			t.Error("Load balancer should return nil result with nil request")
		}

		testLogger.InfoEvent().
			Str("component", "LoadBalancerTest").
			Msg("Load balancer integration test completed")
	})

	t.Run("StreamInstanceIntegration", func(t *testing.T) {
		// Create stream instance configuration
		streamConfig := &config.StreamConfig{
			SharedBufferSize: 5,
			ChunkSize:        512,
			TimeoutSeconds:   15,
			InitialBackoff:   50 * time.Millisecond,
		}

		cm := store.NewConcurrencyManager()
		correlationID := "stream-instance-test-789"
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)
		
		testLogger := logger.Default.WithCorrelationID(ctx)

		testLogger.InfoEvent().
			Str("component", "StreamInstanceTest").
			Str("operation", "instance_creation").
			Msg("Testing stream instance creation")

		// Create stream instance
		streamInstance, err := stream.NewStreamInstance(cm, streamConfig,
			stream.WithLogger(testLogger))

		if err != nil {
			t.Fatalf("Failed to create stream instance: %v", err)
		}

		if streamInstance == nil {
			t.Fatal("Stream instance should not be nil")
		}

		testLogger.InfoEvent().
			Str("component", "StreamInstanceTest").
			Msg("Stream instance created successfully")

		// Test error scenarios
		testLogger.InfoEvent().
			Str("component", "StreamInstanceTest").
			Str("operation", "error_scenarios").
			Msg("Testing stream instance error scenarios")

		// Test with nil concurrency manager
		_, err = stream.NewStreamInstance(nil, streamConfig)
		if err == nil {
			t.Error("Stream instance creation should fail with nil concurrency manager")
		} else {
			testLogger.DebugEvent().
				Str("component", "StreamInstanceTest").
				Err(err).
				Msg("Stream instance correctly failed with nil concurrency manager")
		}

		testLogger.InfoEvent().
			Str("component", "StreamInstanceTest").
			Msg("Stream instance integration test completed")
	})

	t.Run("BufferOperationsLogging", func(t *testing.T) {
		// Test detailed buffer operations with extensive logging
		streamConfig := &config.StreamConfig{
			SharedBufferSize: 3, // Small buffer for testing
			ChunkSize:        256,
			TimeoutSeconds:   5,
			InitialBackoff:   25 * time.Millisecond,
		}

		cm := store.NewConcurrencyManager()
		streamID := "buffer-ops-test"
		correlationID := "buffer-operations-test-999"
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)
		
		testLogger := logger.Default.WithCorrelationID(ctx)
		
		// Create coordinator with contextual logging
		bufferLogger := testLogger.WithFields(map[string]interface{}{
			"component":         "BufferOperationsTest",
			"stream_id":         streamID,
			"buffer_size":       streamConfig.SharedBufferSize,
			"chunk_size":        streamConfig.ChunkSize,
		})

		coordinator := buffer.NewStreamCoordinator(streamID, streamConfig, cm, bufferLogger)

		// Register multiple clients
		bufferLogger.InfoEvent().
			Str("operation", "multi_client_registration").
			Msg("Testing multiple client registration")

		for i := 0; i < 2; i++ {
			err := coordinator.RegisterClient()
			if err != nil {
				t.Errorf("Failed to register client %d: %v", i, err)
			}

			bufferLogger.DebugEvent().
				Int("client_number", i+1).
				Int("total_clients", int(coordinator.ClientCount)).
				Msg("Client registered successfully")
		}

		// Test buffer overflow scenario
		bufferLogger.InfoEvent().
			Str("operation", "buffer_overflow_test").
			Msg("Testing buffer overflow scenario")

		// Write more chunks than buffer size to test overflow handling
		for i := 0; i < streamConfig.SharedBufferSize*2; i++ {
			chunk := &buffer.ChunkData{
				Buffer:    nil,
				Error:     nil,
				Status:    0,
				Timestamp: time.Now(),
			}
			chunk.Reset()

			bufferLogger.DebugEvent().
				Int("chunk_number", i).
				Int("buffer_size", streamConfig.SharedBufferSize).
				Bool("overflow_expected", i >= streamConfig.SharedBufferSize).
				Msg("Writing chunk to test buffer overflow")

			coordinator.Write(chunk)
		}

		// Test reading from buffer
		bufferLogger.InfoEvent().
			Str("operation", "buffer_read_test").
			Msg("Testing buffer read operations")

		chunks, errorChunk, nextPosition := coordinator.ReadChunks(nil)
		
		bufferLogger.InfoEvent().
			Int("chunks_read", len(chunks)).
			Bool("error_found", errorChunk != nil).
			Bool("next_position_set", nextPosition != nil).
			Msg("Buffer read completed")

		// Clean up - unregister all clients
		bufferLogger.InfoEvent().
			Str("operation", "cleanup").
			Msg("Cleaning up test clients")

		for i := 0; i < 2; i++ {
			coordinator.UnregisterClient()
			bufferLogger.DebugEvent().
				Int("remaining_clients", int(coordinator.ClientCount)).
				Msg("Client unregistered")
		}

		bufferLogger.InfoEvent().
			Str("stream_id", streamID).
			Msg("Buffer operations test completed")
	})
}

// TestStreamErrorHandlingIntegration tests error scenarios in stream processing
func TestStreamErrorHandlingIntegration(t *testing.T) {
	t.Run("CoordinatorErrorStates", func(t *testing.T) {
		streamConfig := &config.StreamConfig{
			SharedBufferSize: 3,
			ChunkSize:        512,
			TimeoutSeconds:   5,
			InitialBackoff:   50 * time.Millisecond,
		}

		cm := store.NewConcurrencyManager()
		streamID := "error-handling-test"
		correlationID := "error-handling-test-111"
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)
		
		testLogger := logger.Default.WithCorrelationID(ctx)
		
		errorLogger := testLogger.WithFields(map[string]interface{}{
			"component":   "ErrorHandlingTest",
			"stream_id":   streamID,
			"test_type":   "coordinator_errors",
		})

		coordinator := buffer.NewStreamCoordinator(streamID, streamConfig, cm, errorLogger)

		// Test various error scenarios
		errorLogger.InfoEvent().
			Str("operation", "nil_chunk_test").
			Msg("Testing nil chunk handling")

		// Test writing nil chunk
		success := coordinator.Write(nil)
		if success {
			t.Error("Writing nil chunk should return false")
		}

		errorLogger.DebugEvent().
			Bool("nil_chunk_handled", !success).
			Msg("Nil chunk correctly rejected")

		// Register client and test error chunk scenarios
		coordinator.RegisterClient()

		errorLogger.InfoEvent().
			Str("operation", "error_chunk_scenarios").
			Msg("Testing various error chunk scenarios")

		// Test different types of errors
		errorTypes := []struct {
			name   string
			err    error
			status int
		}{
			{"network_error", fmt.Errorf("network timeout"), 0},
			{"http_error", nil, 404},
			{"server_error", fmt.Errorf("internal server error"), 500},
			{"combined_error", fmt.Errorf("bad gateway"), 502},
		}

		for _, errType := range errorTypes {
			errorLogger.InfoEvent().
				Str("error_type", errType.name).
				Str("error_message", func() string {
					if errType.err != nil {
						return errType.err.Error()
					}
					return ""
				}()).
				Int("status_code", errType.status).
				Msg("Testing specific error type")

			errorChunk := &buffer.ChunkData{
				Buffer:    nil,
				Error:     errType.err,
				Status:    errType.status,
				Timestamp: time.Now(),
			}
			errorChunk.Reset()

			coordinator.Write(errorChunk)
		}

		// Test reading error chunks
		errorLogger.InfoEvent().
			Str("operation", "error_chunk_read").
			Msg("Testing error chunk reading")

		chunks, errorChunk, _ := coordinator.ReadChunks(nil)
		
		errorLogger.InfoEvent().
			Int("normal_chunks", len(chunks)).
			Bool("error_chunk_found", errorChunk != nil).
			Msg("Error chunk read test completed")

		coordinator.UnregisterClient()

		errorLogger.InfoEvent().
			Str("stream_id", streamID).
			Msg("Coordinator error handling test completed")
	})

	t.Run("LoadBalancerErrorScenarios", func(t *testing.T) {
		lbConfig := &loadbalancer.LBConfig{
			MaxRetries: 1,
			RetryWait:  1,
		}

		cm := store.NewConcurrencyManager()
		correlationID := "lb-error-test-222"
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)
		
		testLogger := logger.Default.WithCorrelationID(ctx)
		
		lbErrorLogger := testLogger.WithFields(map[string]interface{}{
			"component":  "LoadBalancerErrorTest",
			"test_type":  "error_scenarios",
			"max_retries": lbConfig.MaxRetries,
		})

		lb := loadbalancer.NewLoadBalancerInstance(cm, lbConfig,
			loadbalancer.WithLogger(lbErrorLogger))

		// Test various error scenarios
		lbErrorLogger.InfoEvent().
			Str("operation", "invalid_requests").
			Msg("Testing invalid request scenarios")

		req := createTestHTTPRequest("GET", "/nonexistent-stream.m3u8")

		// Test with cancelled context
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		result, err := lb.Balance(cancelCtx, req)
		if err == nil {
			t.Error("Load balancer should fail with cancelled context")
		}

		lbErrorLogger.DebugEvent().
			Err(err).
			Bool("context_cancelled", err != nil).
			Msg("Cancelled context correctly handled")

		if result != nil {
			t.Error("Result should be nil with cancelled context")
		}

		// Test timeout scenario
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer timeoutCancel()

		lbErrorLogger.InfoEvent().
			Str("operation", "timeout_test").
			Dur("timeout", 100*time.Millisecond).
			Msg("Testing timeout scenario")

		// This should timeout since we don't have real streams configured
		result2, err2 := lb.Balance(timeoutCtx, req)
		
		lbErrorLogger.DebugEvent().
			Err(err2).
			Bool("timeout_occurred", err2 != nil).
			Msg("Timeout scenario handled")

		if result2 != nil && err2 == nil {
			t.Log("Unexpected success in timeout test (may vary by environment)")
		}

		lbErrorLogger.InfoEvent().
			Msg("Load balancer error scenarios test completed")
	})
}

// TestStreamPerformanceLogging tests performance-related logging in stream processing
func TestStreamPerformanceLogging(t *testing.T) {
	t.Run("BufferPerformanceMetrics", func(t *testing.T) {
		streamConfig := &config.StreamConfig{
			SharedBufferSize: 10,
			ChunkSize:        1024,
			TimeoutSeconds:   30,
			InitialBackoff:   10 * time.Millisecond,
		}

		cm := store.NewConcurrencyManager()
		streamID := "performance-test"
		correlationID := "performance-test-333"
		ctx := correlation.WithCorrelationID(context.Background(), correlationID)
		
		testLogger := logger.Default.WithCorrelationID(ctx)
		
		perfLogger := testLogger.WithFields(map[string]interface{}{
			"component":     "PerformanceTest",
			"stream_id":     streamID,
			"test_type":     "buffer_performance",
			"buffer_size":   streamConfig.SharedBufferSize,
			"chunk_size":    streamConfig.ChunkSize,
		})

		coordinator := buffer.NewStreamCoordinator(streamID, streamConfig, cm, perfLogger)
		coordinator.RegisterClient()

		// Test write performance
		perfLogger.InfoEvent().
			Str("operation", "write_performance").
			Msg("Starting buffer write performance test")

		writeCount := 100
		startTime := time.Now()

		for i := 0; i < writeCount; i++ {
			chunk := &buffer.ChunkData{
				Buffer:    nil,
				Error:     nil,
				Status:    0,
				Timestamp: time.Now(),
			}
			chunk.Reset()

			coordinator.Write(chunk)

			// Log every 25 writes
			if (i+1)%25 == 0 {
				elapsed := time.Since(startTime)
				perfLogger.DebugEvent().
					Int("writes_completed", i+1).
					Dur("elapsed_time", elapsed).
					Float64("writes_per_second", float64(i+1)/elapsed.Seconds()).
					Msg("Write performance checkpoint")
			}
		}

		writeElapsed := time.Since(startTime)
		writesPerSecond := float64(writeCount) / writeElapsed.Seconds()

		perfLogger.InfoEvent().
			Int("total_writes", writeCount).
			Dur("total_time", writeElapsed).
			Float64("writes_per_second", writesPerSecond).
			Msg("Buffer write performance test completed")

		// Test read performance
		perfLogger.InfoEvent().
			Str("operation", "read_performance").
			Msg("Starting buffer read performance test")

		readCount := 50
		readStartTime := time.Now()

		for i := 0; i < readCount; i++ {
			chunks, _, _ := coordinator.ReadChunks(nil)
			
			if (i+1)%10 == 0 {
				elapsed := time.Since(readStartTime)
				perfLogger.DebugEvent().
					Int("reads_completed", i+1).
					Int("chunks_in_last_read", len(chunks)).
					Dur("elapsed_time", elapsed).
					Float64("reads_per_second", float64(i+1)/elapsed.Seconds()).
					Msg("Read performance checkpoint")
			}
		}

		readElapsed := time.Since(readStartTime)
		readsPerSecond := float64(readCount) / readElapsed.Seconds()

		perfLogger.InfoEvent().
			Int("total_reads", readCount).
			Dur("total_time", readElapsed).
			Float64("reads_per_second", readsPerSecond).
			Msg("Buffer read performance test completed")

		coordinator.UnregisterClient()

		perfLogger.InfoEvent().
			Str("stream_id", streamID).
			Dur("total_test_time", time.Since(startTime)).
			Msg("Buffer performance test completed")
	})
}

// Helper function to create test HTTP requests
func createTestHTTPRequest(method, path string) *http.Request {
	req, _ := http.NewRequest(method, path, nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("User-Agent", "StreamIntegrationTest/1.0")
	return req
}