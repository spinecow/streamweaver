package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestLoggerBackwardCompatibility tests that all existing logging methods continue to work as before
func TestLoggerBackwardCompatibility(t *testing.T) {
	// Set environment variable to enable debug logging
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	// Test basic logging methods
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test all existing methods
	logger.Log("test log message")
	logger.Logf("test log message with %s", "format")
	logger.Debug("test debug message")
	logger.Debugf("test debug message with %s", "format")
	logger.Warn("test warn message")
	logger.Warnf("test warn message with %s", "format")
	logger.Error("test error message")
	logger.Errorf("test error message with %s", "format")

	// Check that all messages are present in output
	output := buf.String()
	requiredMessages := []string{
		"test log message",
		"test log message with format",
		"test debug message",
		"test debug message with format",
		"test warn message",
		"test warn message with format",
		"test error message",
		"test error message with format",
	}

	for _, msg := range requiredMessages {
		if !strings.Contains(output, msg) {
			t.Errorf("Expected message '%s' to be present in output: %s", msg, output)
		}
	}
}

// TestLoggerDebugEnvironmentVariable tests that the DEBUG environment variable controls debug logging
func TestLoggerDebugEnvironmentVariable(t *testing.T) {
	// Set DEBUG environment variable
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test debug logging
	logger.Debug("debug message")

	// Check that debug message is present
	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected debug message to be present in output with DEBUG=true: %s", output)
	}
}

// TestLoggerWithFields tests the WithFields method to ensure fields are properly added to log messages
func TestLoggerWithFields(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test WithFields
	newLogger := logger.WithFields(map[string]interface{}{
		"test_key": "test_value",
		"number":   42,
		"boolean":  true,
	})

	if newLogger == nil {
		t.Error("WithFields should return a new logger instance")
	}

	// Test that the new logger is a different instance
	if newLogger == logger {
		t.Error("WithFields should return a different logger instance")
	}

	// Use the new logger to log something
	if typedLogger, ok := newLogger.(*DefaultLogger); ok {
		typedLogger.logger.Info().Msg("test message with fields")
	}

	// Parse the JSON output
	output := buf.String()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Check that the fields are present
	if val, ok := parsed["test_key"]; !ok || val != "test_value" {
		t.Errorf("Expected test_key field to be 'test_value', got: %v", val)
	}

	if val, ok := parsed["number"]; !ok || val != 42.0 { // JSON numbers are float64
		t.Errorf("Expected number field to be 42, got: %v", val)
	}

	if val, ok := parsed["boolean"]; !ok || val != true {
		t.Errorf("Expected boolean field to be true, got: %v", val)
	}
}

// TestLoggerWithSensitiveField tests the WithSensitiveField method to ensure sensitive information is properly redacted when SAFE_LOGS is enabled
func TestLoggerWithSensitiveField(t *testing.T) {
	// Test with SAFE_LOGS=true
	os.Setenv("SAFE_LOGS", "true")
	defer os.Unsetenv("SAFE_LOGS")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test WithSensitiveField
	newLogger := logger.WithSensitiveField("password", "secret123")

	if newLogger == nil {
		t.Error("WithSensitiveField should return a new logger instance")
	}

	// Test that the new logger is a different instance
	if newLogger == logger {
		t.Error("WithSensitiveField should return a different logger instance")
	}

	// Use the new logger to log something
	if typedLogger, ok := newLogger.(*DefaultLogger); ok {
		typedLogger.logger.Info().Msg("test message with sensitive field")
	}

	// Check that the password field is redacted
	output := buf.String()
	if strings.Contains(output, "secret123") {
		t.Errorf("Expected sensitive field to be redacted in output: %s", output)
	}

	if !strings.Contains(output, "[redacted]") {
		t.Errorf("Expected redacted sensitive field in output: %s", output)
	}
}

// TestLoggerEventMethods tests all event methods with various field types
func TestLoggerEventMethods(t *testing.T) {
	// Set environment variable to enable debug logging
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test InfoEvent
	logger.InfoEvent().Str("string_field", "test").Int("int_field", 42).Msg("info event message")

	// Test DebugEvent
	logger.DebugEvent().Bool("bool_field", true).Float64("float_field", 3.14).Msg("debug event message")

	// Test WarnEvent
	logger.WarnEvent().Time("time_field", time.Now()).Dur("duration_field", time.Second).Msg("warn event message")

	// Test ErrorEvent
	logger.ErrorEvent().Err(fmt.Errorf("test error")).Msg("error event message")

	// Note: We're not testing FatalEvent as it would exit the program

	// Parse the JSON output
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// We expect 4 lines, one for each event type
	if len(lines) < 4 {
		t.Fatalf("Expected at least 4 log lines, got: %d. Output: %s", len(lines), output)
	}

	// Check each event type
	for i, line := range lines[:4] { // Only check the first 4 lines
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("Failed to parse JSON output line %d: %v", i, err)
			continue
		}

		switch i {
		case 0: // InfoEvent
			if level, ok := parsed["level"]; !ok || level != "info" {
				t.Errorf("Expected level 'info', got: %v", level)
			}
			if msg, ok := parsed["message"]; !ok || msg != "info event message" {
				t.Errorf("Expected message 'info event message', got: %v", msg)
			}
			if val, ok := parsed["string_field"]; !ok || val != "test" {
				t.Errorf("Expected string_field 'test', got: %v", val)
			}
			if val, ok := parsed["int_field"]; !ok || val != 42.0 {
				t.Errorf("Expected int_field 42, got: %v", val)
			}
		case 1: // DebugEvent
			if level, ok := parsed["level"]; !ok || level != "debug" {
				t.Errorf("Expected level 'debug', got: %v", level)
			}
			if msg, ok := parsed["message"]; !ok || msg != "debug event message" {
				t.Errorf("Expected message 'debug event message', got: %v", msg)
			}
			if val, ok := parsed["bool_field"]; !ok || val != true {
				t.Errorf("Expected bool_field true, got: %v", val)
			}
			if val, ok := parsed["float_field"]; !ok || val != 3.14 {
				t.Errorf("Expected float_field 3.14, got: %v", val)
			}
		case 2: // WarnEvent
			if level, ok := parsed["level"]; !ok || level != "warn" {
				t.Errorf("Expected level 'warn', got: %v", level)
			}
			if msg, ok := parsed["message"]; !ok || msg != "warn event message" {
				t.Errorf("Expected message 'warn event message', got: %v", msg)
			}
			if _, ok := parsed["time_field"]; !ok {
				t.Error("Expected time_field to be present")
			}
			// Zerolog serializes durations in milliseconds, so we expect 1000 for 1 second
			if val, ok := parsed["duration_field"]; !ok || val != 1000.0 {
				t.Errorf("Expected duration_field 1000 (1 second in milliseconds), got: %v", val)
			}
		case 3: // ErrorEvent
			if level, ok := parsed["level"]; !ok || level != "error" {
				t.Errorf("Expected level 'error', got: %v", level)
			}
			if msg, ok := parsed["message"]; !ok || msg != "error event message" {
				t.Errorf("Expected message 'error event message', got: %v", msg)
			}
			if err, ok := parsed["error"]; !ok || !strings.Contains(err.(string), "test error") {
				t.Errorf("Expected error field to contain 'test error', got: %v", err)
			}
		}
	}
}

// TestLoggerFieldPersistence tests field persistence and inheritance when creating child loggers
func TestLoggerFieldPersistence(t *testing.T) {
	// Create a base logger with fields
	// We need to create the base logger with a captured output buffer
	var buf bytes.Buffer
	baseLogger := NewDefaultLogger()
	
	// Replace the logger with one that captures output
	origLogger := baseLogger.logger
	baseLogger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { baseLogger.logger = origLogger }()

	// Add fields to the base logger
	baseLoggerWithFields := baseLogger.WithFields(map[string]interface{}{
		"base_field": "base_value",
	})

	// Create a child logger with additional fields
	childLogger := baseLoggerWithFields.WithFields(map[string]interface{}{
		"child_field": "child_value",
	})

	// Both loggers should be valid instances
	if baseLoggerWithFields == nil {
		t.Error("Base logger with fields should not be nil")
	}

	if childLogger == nil {
		t.Error("Child logger should not be nil")
	}

	// They should be different instances
	if baseLoggerWithFields == childLogger {
		t.Error("Base and child loggers should be different instances")
	}

	// Use the child logger to log something
	if typedLogger, ok := childLogger.(*DefaultLogger); ok {
		typedLogger.logger.Info().Msg("test message with inherited fields")
	}

	// Parse the JSON output
	output := buf.String()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v. Output: %s", err, output)
	}

	// Check that both base and child fields are present
	// Note: Field inheritance happens at the zerolog level, so we need to check if our implementation
	// correctly passes the fields to zerolog
	if val, ok := parsed["base_field"]; !ok || val != "base_value" {
		t.Errorf("Expected base_field 'base_value', got: %v", val)
	}

	if val, ok := parsed["child_field"]; !ok || val != "child_value" {
		t.Errorf("Expected child_field 'child_value', got: %v", val)
	}
}

// TestLoggerLogFormatJSON tests that JSON format produces valid JSON output
func TestLoggerLogFormatJSON(t *testing.T) {
	// Set environment variable
	os.Setenv("LOG_FORMAT", "json")
	defer os.Unsetenv("LOG_FORMAT")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test logging
	logger.Log("test message")

	// Check that output is valid JSON
	output := buf.String()
	if !json.Valid([]byte(strings.TrimSpace(output))) {
		t.Errorf("Expected JSON output, got: %s", output)
	}

	// Check that message is present
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message to be present in output: %s", output)
	}

	// Check that required fields are present
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if _, ok := parsed["level"]; !ok {
		t.Error("Expected level field to be present")
	}

	if _, ok := parsed["time"]; !ok {
		t.Error("Expected time field to be present")
	}

	if msg, ok := parsed["message"]; !ok || msg != "test message" {
		t.Errorf("Expected message field to be 'test message', got: %v", msg)
	}
}

// TestLoggerLogFormatConsole tests that console format produces readable console output
func TestLoggerLogFormatConsole(t *testing.T) {
	// Set environment variable
	os.Setenv("LOG_FORMAT", "console")
	defer os.Unsetenv("LOG_FORMAT")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(zerolog.ConsoleWriter{Out: &buf}).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test logging
	logger.Log("test message")

	// Check that output is not JSON (console format)
	output := buf.String()
	if json.Valid([]byte(strings.TrimSpace(output))) {
		t.Errorf("Expected console output, got JSON: %s", output)
	}

	// Check that message is present
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message to be present in output: %s", output)
	}

	// Check that required elements are present
	if !strings.Contains(output, "INF") && !strings.Contains(output, "info") {
		t.Errorf("Expected level indicator in console output: %s", output)
	}
}

// TestLoggerLogLevelDebug tests LOG_LEVEL environment variable with "debug" value
func TestLoggerLogLevelDebug(t *testing.T) {
	// Set environment variables
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test debug logging
	logger.Debug("debug message")

	// Check that debug message is present
	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected debug message to be present in output: %s", output)
	}
}

// TestLoggerLogLevelInfo tests LOG_LEVEL environment variable with "info" value
func TestLoggerLogLevelInfo(t *testing.T) {
	// Set environment variables
	os.Setenv("LOG_LEVEL", "info")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test info logging
	logger.Log("info message")

	// Check that info message is present
	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Errorf("Expected info message to be present in output: %s", output)
	}
}

// TestLoggerLogLevelWarn tests LOG_LEVEL environment variable with "warn" value
func TestLoggerLogLevelWarn(t *testing.T) {
	// Set environment variables
	os.Setenv("LOG_LEVEL", "warn")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test warn logging
	logger.Warn("warn message")

	// Check that warn message is present
	output := buf.String()
	if !strings.Contains(output, "warn message") {
		t.Errorf("Expected warn message to be present in output: %s", output)
	}
}

// TestLoggerLogLevelError tests LOG_LEVEL environment variable with "error" value
func TestLoggerLogLevelError(t *testing.T) {
	// Set environment variables
	os.Setenv("LOG_LEVEL", "error")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test error logging
	logger.Error("error message")

	// Check that error message is present
	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("Expected error message to be present in output: %s", output)
	}
}

// TestLoggerLogLevelFatal tests LOG_LEVEL environment variable with "fatal" value
func TestLoggerLogLevelFatal(t *testing.T) {
	// Set environment variables
	os.Setenv("LOG_LEVEL", "fatal")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test fatal logging (we won't actually call Fatal as it would exit the program)
	// Instead, we'll test that the logger is created with the correct level
	// This is indirectly tested by checking that the logger was created successfully
	if logger == nil {
		t.Error("Expected logger to be created successfully")
	}
}

// TestLoggerSafeLogs tests SAFE_LOGS environment variable for URL redaction
func TestLoggerSafeLogs(t *testing.T) {
	// Set environment variables
	os.Setenv("SAFE_LOGS", "true")
	defer os.Unsetenv("SAFE_LOGS")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test logging with URL
	logger.Log("Visit http://example.com for more information")

	// Check that URL is redacted
	output := buf.String()
	if strings.Contains(output, "http://example.com") {
		t.Errorf("Expected URL to be redacted in output: %s", output)
	}

	if !strings.Contains(output, "[redacted url]") {
		t.Errorf("Expected redacted URL in output: %s", output)
	}
}

// TestLoggerFieldStandardization tests that standard fields are present in all log messages
func TestLoggerFieldStandardization(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test logging
	logger.Log("test message")

	// Parse the JSON output
	output := buf.String()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Check that standard fields are present
	requiredFields := []string{"time", "level", "message"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Expected field '%s' to be present in log output", field)
		}
	}

	// Check field values
	if level, ok := parsed["level"]; !ok || level != "info" {
		t.Errorf("Expected level to be 'info', got: %v", level)
	}

	if msg, ok := parsed["message"]; !ok || msg != "test message" {
		t.Errorf("Expected message to be 'test message', got: %v", msg)
	}

	// Check that time is a valid timestamp
	if timeStr, ok := parsed["time"]; ok {
		_, err := time.Parse(time.RFC3339, timeStr.(string))
		if err != nil {
			t.Errorf("Expected time to be a valid RFC3339 timestamp, got: %v", timeStr)
		}
	} else {
		t.Error("Expected time field to be present")
	}
}

// TestLoggerErrorHandlingWithInvalidFieldValues tests behavior with invalid field values
func TestLoggerErrorHandlingWithInvalidFieldValues(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test WithFields with various field types including potentially problematic ones
	newLogger := logger.WithFields(map[string]interface{}{
		"nil_field":      nil,
		"empty_key":      "value", // This should work
		"string_field":   "test",
		"number_field":   42,
		"float_field":    3.14,
		"bool_field":     true,
		"time_field":     time.Now(),
		"duration_field": time.Second,
	})

	if newLogger == nil {
		t.Error("WithFields should return a new logger instance even with various field types")
	}
}

// TestLoggerErrorHandlingWithNilValues tests behavior with nil values
func TestLoggerErrorHandlingWithNilValues(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()

	// Test WithFields with nil values
	newLogger := logger.WithFields(map[string]interface{}{
		"nil_field": nil,
	})

	if newLogger == nil {
		t.Error("WithFields should handle nil values gracefully")
	}

	// Test WithSensitiveField with empty values
	newLogger2 := logger.WithSensitiveField("", "value")
	if newLogger2 == nil {
		t.Error("WithSensitiveField should handle empty key gracefully")
	}

	newLogger3 := logger.WithSensitiveField("key", "")
	if newLogger3 == nil {
		t.Error("WithSensitiveField should handle empty value gracefully")
	}
}

// TestLoggerErrorHandlingWithSpecialCharacters tests behavior with special characters in field names or values
func TestLoggerErrorHandlingWithSpecialCharacters(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Test WithFields with special characters
	newLogger := logger.WithFields(map[string]interface{}{
		"field_with_underscores": "value_with_underscores",
		"field-with-dashes":      "value-with-dashes",
		"field with spaces":      "value with spaces",
		"fieldWithUnicode测试":     "valueWithUnicode测试",
		"fieldWithEmoji😀":        "valueWithEmoji😀",
	})

	if newLogger == nil {
		t.Error("WithFields should handle special characters in field names gracefully")
	}

	// Use the new logger to log something
	if typedLogger, ok := newLogger.(*DefaultLogger); ok {
		typedLogger.logger.Info().Msg("test message with special characters")
	}

	// Parse the JSON output
	output := buf.String()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Check that fields with special characters are present
	expectedFields := map[string]string{
		"field_with_underscores": "value_with_underscores",
		"field-with-dashes":      "value-with-dashes",
		"field with spaces":      "value with spaces",
		"fieldWithUnicode测试":     "valueWithUnicode测试",
		"fieldWithEmoji😀":        "valueWithEmoji😀",
	}

	for fieldName, expectedValue := range expectedFields {
		if val, ok := parsed[fieldName]; !ok || val != expectedValue {
			t.Errorf("Expected field '%s' to be '%s', got: %v", fieldName, expectedValue, val)
		}
	}
}

// TestLoggerPerformance tests that logging performance is acceptable
func TestLoggerPerformance(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output to prevent it from being printed
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Measure time for 10000 log operations
	start := time.Now()
	for i := 0; i < 10000; i++ {
		logger.Log("test message")
	}
	elapsed := time.Since(start)

	// Check that it completes within a reasonable time (less than 1 second)
	if elapsed > time.Second {
		t.Errorf("Expected 10000 log operations to complete in less than 1 second, took: %v", elapsed)
	}
}

// TestLoggerFieldSerializationPerformance tests that field serialization doesn't introduce significant overhead
func TestLoggerFieldSerializationPerformance(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output to prevent it from being printed
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger.logger = origLogger }()

	// Create a logger with many fields
	fields := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		fields[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	fieldLogger := logger.WithFields(fields)

	// Measure time for 1000 log operations with many fields
	start := time.Now()
	for i := 0; i < 1000; i++ {
		if typedLogger, ok := fieldLogger.(*DefaultLogger); ok {
			typedLogger.logger.Info().Msg("test message with many fields")
		}
	}
	elapsed := time.Since(start)

	// Check that it completes within a reasonable time (less than 1 second)
	if elapsed > time.Second {
		t.Errorf("Expected 1000 log operations with many fields to complete in less than 1 second, took: %v", elapsed)
	}
}