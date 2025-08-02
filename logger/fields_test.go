package logger

import (
	"testing"
	"time"
)

func TestStandardFields_RequestResponseFields(t *testing.T) {
	fields := NewStandardFields()
	
	fields.WithCorrelationID("test-123").
		WithMethod("GET").
		WithURL("http://example.com").
		WithPath("/api/test").
		WithStatusCode(200).
		WithDuration(time.Millisecond * 100).
		WithClientIP("192.168.1.1").
		WithUserAgent("test-agent")
	
	f := fields.Fields()
	
	if f["correlation_id"] != "test-123" {
		t.Errorf("Expected correlation_id to be test-123, got %v", f["correlation_id"])
	}
	
	if f["method"] != "GET" {
		t.Errorf("Expected method to be GET, got %v", f["method"])
	}
	
	if f["status_code"] != 200 {
		t.Errorf("Expected status_code to be 200, got %v", f["status_code"])
	}
	
	if f["duration"] != time.Millisecond*100 {
		t.Errorf("Expected duration to be 100ms, got %v", f["duration"])
	}
}

func TestStandardFields_ComponentFields(t *testing.T) {
	fields := NewStandardFields()
	
	fields.WithComponent("stream-handler").
		WithOperation("process-stream").
		WithError("connection failed").
		WithErrorType("network_error")
	
	f := fields.Fields()
	
	if f["component"] != "stream-handler" {
		t.Errorf("Expected component to be stream-handler, got %v", f["component"])
	}
	
	if f["operation"] != "process-stream" {
		t.Errorf("Expected operation to be process-stream, got %v", f["operation"])
	}
	
	if f["error"] != "connection failed" {
		t.Errorf("Expected error to be 'connection failed', got %v", f["error"])
	}
	
	if f["error_type"] != "network_error" {
		t.Errorf("Expected error_type to be network_error, got %v", f["error_type"])
	}
}

func TestStandardFields_StreamFields(t *testing.T) {
	fields := NewStandardFields()
	
	fields.WithStreamID("stream-456").
		WithChannelName("test-channel").
		WithPlaylistURL("http://example.com/playlist.m3u8").
		WithSegmentURL("http://example.com/segment1.ts").
		WithBufferStatus("buffering").
		WithClientCount(5).
		WithStreamType("m3u8")
	
	f := fields.Fields()
	
	if f["stream_id"] != "stream-456" {
		t.Errorf("Expected stream_id to be stream-456, got %v", f["stream_id"])
	}
	
	if f["channel_name"] != "test-channel" {
		t.Errorf("Expected channel_name to be test-channel, got %v", f["channel_name"])
	}
	
	if f["client_count"] != 5 {
		t.Errorf("Expected client_count to be 5, got %v", f["client_count"])
	}
	
	if f["stream_type"] != "m3u8" {
		t.Errorf("Expected stream_type to be m3u8, got %v", f["stream_type"])
	}
}

func TestStandardFields_LoadBalancerFields(t *testing.T) {
	fields := NewStandardFields()
	
	fields.WithLoadBalancerResult("success").
		WithTargetURL("http://target.example.com").
		WithAttemptCount(3).
		WithFailoverReason("timeout")
	
	f := fields.Fields()
	
	if f["load_balancer_result"] != "success" {
		t.Errorf("Expected load_balancer_result to be success, got %v", f["load_balancer_result"])
	}
	
	if f["target_url"] != "http://target.example.com" {
		t.Errorf("Expected target_url to be http://target.example.com, got %v", f["target_url"])
	}
	
	if f["attempt_count"] != 3 {
		t.Errorf("Expected attempt_count to be 3, got %v", f["attempt_count"])
	}
	
	if f["failover_reason"] != "timeout" {
		t.Errorf("Expected failover_reason to be timeout, got %v", f["failover_reason"])
	}
}

func TestStandardFields_PerformanceFields(t *testing.T) {
	fields := NewStandardFields()
	
	fields.WithLatency(time.Millisecond * 50).
		WithBytesTransferred(1024).
		WithResponseSize(2048).
		WithRequestSize(512)
	
	f := fields.Fields()
	
	if f["latency"] != time.Millisecond*50 {
		t.Errorf("Expected latency to be 50ms, got %v", f["latency"])
	}
	
	if f["bytes_transferred"] != int64(1024) {
		t.Errorf("Expected bytes_transferred to be 1024, got %v", f["bytes_transferred"])
	}
	
	if f["response_size"] != int64(2048) {
		t.Errorf("Expected response_size to be 2048, got %v", f["response_size"])
	}
	
	if f["request_size"] != int64(512) {
		t.Errorf("Expected request_size to be 512, got %v", f["request_size"])
	}
}

func TestStandardFields_SystemFields(t *testing.T) {
	fields := NewStandardFields()
	
	fields.WithMemoryUsage(1048576).
		WithCPUUsage(75.5).
		WithGoroutineCount(10)
	
	f := fields.Fields()
	
	if f["memory_usage"] != int64(1048576) {
		t.Errorf("Expected memory_usage to be 1048576, got %v", f["memory_usage"])
	}
	
	if f["cpu_usage"] != 75.5 {
		t.Errorf("Expected cpu_usage to be 75.5, got %v", f["cpu_usage"])
	}
	
	if f["goroutine_count"] != 10 {
		t.Errorf("Expected goroutine_count to be 10, got %v", f["goroutine_count"])
	}
}

func TestStandardFields_TimeField(t *testing.T) {
	fields := NewStandardFields()
	testTime := time.Now()
	
	fields.WithTime("created_at", testTime)
	
	f := fields.Fields()
	
	if f["created_at"] != testTime {
		t.Errorf("Expected created_at to be %v, got %v", testTime, f["created_at"])
	}
}

func TestStandardFields_GenericField(t *testing.T) {
	fields := NewStandardFields()
	
	fields.WithField("custom_field", "custom_value")
	
	f := fields.Fields()
	
	if f["custom_field"] != "custom_value" {
		t.Errorf("Expected custom_field to be custom_value, got %v", f["custom_field"])
	}
}

func TestStandardFields_Chaining(t *testing.T) {
	fields := NewStandardFields().
		WithCorrelationID("test-123").
		WithMethod("POST").
		WithComponent("handler").
		WithStreamID("stream-456")
	
	f := fields.Fields()
	
	// Verify all fields are present
	expectedFields := []string{"correlation_id", "method", "component", "stream_id"}
	for _, field := range expectedFields {
		if _, exists := f[field]; !exists {
			t.Errorf("Expected field %s to exist", field)
		}
	}
	
	if len(f) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(f))
	}
}

func TestWithStandardFields_Integration(t *testing.T) {
	logger := NewDefaultLogger()
	
	// Create standardized fields
	standardFields := NewStandardFields().
		WithComponent("test-component").
		WithOperation("test-operation").
		WithCorrelationID("test-correlation-123")
	
	// Create logger with standardized fields
	loggerWithFields := logger.WithStandardFields(standardFields)
	
	if loggerWithFields == nil {
		t.Error("WithStandardFields should return a new logger instance")
	}
	
	// Test that the logger has the fields
	if defaultLogger, ok := loggerWithFields.(*DefaultLogger); ok {
		if defaultLogger.fields["component"] != "test-component" {
			t.Errorf("Expected component field to be 'test-component', got %v", defaultLogger.fields["component"])
		}
		if defaultLogger.fields["operation"] != "test-operation" {
			t.Errorf("Expected operation field to be 'test-operation', got %v", defaultLogger.fields["operation"])
		}
		if defaultLogger.fields["correlation_id"] != "test-correlation-123" {
			t.Errorf("Expected correlation_id field to be 'test-correlation-123', got %v", defaultLogger.fields["correlation_id"])
		}
	} else {
		t.Error("Expected logger to be of type *DefaultLogger")
	}
}

func TestWithStandardFields_NilFields(t *testing.T) {
	logger := NewDefaultLogger()
	
	// Test with nil fields
	loggerWithNilFields := logger.WithStandardFields(nil)
	
	if loggerWithNilFields != logger {
		t.Error("WithStandardFields with nil should return the same logger instance")
	}
}

func TestStandardFields_ApplyToEvent(t *testing.T) {
	// Create an event from a logger
	logger := NewDefaultLogger()
	event := logger.InfoEvent()
	
	// Create standardized fields
	fields := NewStandardFields().
		WithComponent("test-component").
		WithOperation("test-operation").
		WithStatusCode(200)
	
	// Apply fields to event
	eventWithFields := fields.ApplyToEvent(event)
	
	if eventWithFields == nil {
		t.Error("ApplyToEvent should return an event")
	}
	
	// Note: We can't easily test the event content without logging, 
	// but we can verify that the method doesn't panic and returns an event
}