package logger

import (
	"testing"
)

func TestWithFields(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()
	
	// Test WithFields
	newLogger := logger.WithFields(map[string]interface{}{
		"test_key": "test_value",
		"number":   42,
	})
	
	if newLogger == nil {
		t.Error("WithFields should return a new logger instance")
	}
	
	// Test that the new logger is a different instance
	if newLogger == logger {
		t.Error("WithFields should return a different logger instance")
	}
}

func TestWithSensitiveField(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()
	
	// Test WithSensitiveField
	newLogger := logger.WithSensitiveField("password", "secret123")
	
	if newLogger == nil {
		t.Error("WithSensitiveField should return a new logger instance")
	}
	
	// Test that the new logger is a different instance
	if newLogger == logger {
		t.Error("WithSensitiveField should return a different logger instance")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Create a new logger
	logger := NewDefaultLogger()
	
	// Test that all existing methods are still available
	logger.Log("test log message")
	logger.Logf("test log message with %s", "format")
	logger.Debug("test debug message")
	logger.Debugf("test debug message with %s", "format")
	logger.Warn("test warn message")
	logger.Warnf("test warn message with %s", "format")
	logger.Error("test error message")
	logger.Errorf("test error message with %s", "format")
	
	// These would exit the program, so we won't test them
	// logger.Fatal("test fatal message")
	// logger.Fatalf("test fatal message with %s", "format")
	
	// Test event methods
	logger.InfoEvent().Msg("test info event")
	logger.DebugEvent().Msg("test debug event")
	logger.WarnEvent().Msg("test warn event")
	logger.ErrorEvent().Msg("test error event")
	// logger.FatalEvent().Msg("test fatal event") // Would exit program
}