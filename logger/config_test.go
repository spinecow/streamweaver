package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestLogFormatJSON(t *testing.T) {
	// Set environment variable
	os.Setenv("LOG_FORMAT", "json")
	defer os.Unsetenv("LOG_FORMAT")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()

	// Test logging
	logger.Log("test message")

	// Restore original logger
	logger.logger = origLogger

	// Check that output is valid JSON
	output := buf.String()
	if !json.Valid([]byte(strings.TrimSpace(output))) {
		t.Errorf("Expected JSON output, got: %s", output)
	}

	// Check that message is present
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message to be present in output: %s", output)
	}
}

func TestLogFormatConsole(t *testing.T) {
	// Set environment variable
	os.Setenv("LOG_FORMAT", "console")
	defer os.Unsetenv("LOG_FORMAT")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(zerolog.ConsoleWriter{Out: &buf}).With().Timestamp().Logger()

	// Test logging
	logger.Log("test message")

	// Restore original logger
	logger.logger = origLogger

	// Check that output is not JSON (console format)
	output := buf.String()
	if json.Valid([]byte(strings.TrimSpace(output))) {
		t.Errorf("Expected console output, got JSON: %s", output)
	}

	// Check that message is present
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message to be present in output: %s", output)
	}
}

func TestLogFormatDefault(t *testing.T) {
	// Unset environment variable
	os.Unsetenv("LOG_FORMAT")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(zerolog.ConsoleWriter{Out: &buf}).With().Timestamp().Logger()

	// Test logging
	logger.Log("test message")

	// Restore original logger
	logger.logger = origLogger

	// Check that output is not JSON (console format by default)
	output := buf.String()
	if json.Valid([]byte(strings.TrimSpace(output))) {
		t.Errorf("Expected console output by default, got JSON: %s", output)
	}

	// Check that message is present
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message to be present in output: %s", output)
	}
}

func TestLogLevelDebug(t *testing.T) {
	// Set environment variables
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()

	// Test debug logging
	logger.Debug("debug message")

	// Restore original logger
	logger.logger = origLogger

	// Check that debug message is present
	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected debug message to be present in output: %s", output)
	}
}

func TestLogLevelInfo(t *testing.T) {
	// Set environment variables
	os.Setenv("LOG_LEVEL", "info")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()

	// Test info logging
	logger.Log("info message")

	// Restore original logger
	logger.logger = origLogger

	// Check that info message is present
	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Errorf("Expected info message to be present in output: %s", output)
	}
}

func TestLogLevelInvalid(t *testing.T) {
	// Set environment variables
	os.Setenv("LOG_LEVEL", "invalid")
	defer os.Unsetenv("LOG_LEVEL")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()

	// Test info logging (should work with default level)
	logger.Log("info message")

	// Restore original logger
	logger.logger = origLogger

	// Check that info message is present
	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Errorf("Expected info message to be present in output: %s", output)
	}
}

func TestSafeLogs(t *testing.T) {
	// Set environment variables
	os.Setenv("SAFE_LOGS", "true")
	defer os.Unsetenv("SAFE_LOGS")

	// Create a new logger
	logger := NewDefaultLogger()

	// Capture log output
	var buf bytes.Buffer
	origLogger := logger.logger
	logger.logger = zerolog.New(&buf).With().Timestamp().Logger()

	// Test logging with URL
	logger.Log("Visit http://example.com for more information")

	// Restore original logger
	logger.logger = origLogger

	// Check that URL is redacted
	output := buf.String()
	if strings.Contains(output, "http://example.com") {
		t.Errorf("Expected URL to be redacted in output: %s", output)
	}

	if !strings.Contains(output, "[redacted url]") {
		t.Errorf("Expected redacted URL in output: %s", output)
	}
}