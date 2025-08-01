package logger

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/rs/zerolog"
)

type DefaultLogger struct {
	logger zerolog.Logger
	fields map[string]interface{}
}

var Default = NewDefaultLogger()

func NewDefaultLogger() *DefaultLogger {
	var logger zerolog.Logger
	if os.Getenv("LOG_FORMAT") == "json" {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	}
	return &DefaultLogger{
		logger: logger,
		fields: make(map[string]interface{}),
	}
}

// WithFields creates a new logger instance with additional fields
func (l *DefaultLogger) WithFields(fields map[string]interface{}) Logger {
	// Validate fields
	for key, value := range fields {
		// Ensure key is a valid string for JSON
		if key == "" {
			// Skip empty keys
			continue
		}
		// Handle special values that might not serialize well
		if value == nil {
			fields[key] = nil
		}
	}

	// Create a new logger with additional fields
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	newLogger := &DefaultLogger{
		logger: l.logger.With().Fields(newFields).Logger(),
		fields: newFields,
	}
	return newLogger
}

// WithSensitiveField creates a logger with automatic redaction of sensitive fields
func (l *DefaultLogger) WithSensitiveField(key, value string) Logger {
	safeLogs := os.Getenv("SAFE_LOGS") == "true"
	safeValue := value
	if safeLogs {
		safeValue = "[redacted]"
	}

	return l.WithFields(map[string]interface{}{key: safeValue})
}

// EventWrapper wraps a zerolog.Event to implement our Event interface
type EventWrapper struct {
	event *zerolog.Event
}

// Str implements the Event interface for EventWrapper
func (e *EventWrapper) Str(key, val string) Event {
	return &EventWrapper{event: e.event.Str(key, val)}
}

// Int implements the Event interface for EventWrapper
func (e *EventWrapper) Int(key string, i int) Event {
	return &EventWrapper{event: e.event.Int(key, i)}
}

// Int64 implements the Event interface for EventWrapper
func (e *EventWrapper) Int64(key string, i int64) Event {
	return &EventWrapper{event: e.event.Int64(key, i)}
}

// Float64 implements the Event interface for EventWrapper
func (e *EventWrapper) Float64(key string, f float64) Event {
	return &EventWrapper{event: e.event.Float64(key, f)}
}

// Bool implements the Event interface for EventWrapper
func (e *EventWrapper) Bool(key string, b bool) Event {
	return &EventWrapper{event: e.event.Bool(key, b)}
}

// Time implements the Event interface for EventWrapper
func (e *EventWrapper) Time(key string, t time.Time) Event {
	return &EventWrapper{event: e.event.Time(key, t)}
}

// Dur implements the Event interface for EventWrapper
func (e *EventWrapper) Dur(key string, d time.Duration) Event {
	return &EventWrapper{event: e.event.Dur(key, d)}
}

// Err implements the Event interface for EventWrapper
func (e *EventWrapper) Err(err error) Event {
	return &EventWrapper{event: e.event.Err(err)}
}

// Msg implements the Event interface for EventWrapper
func (e *EventWrapper) Msg(msg string) {
	e.event.Msg(safeLogf("%s", msg))
}

// Msgf implements the Event interface for EventWrapper
func (e *EventWrapper) Msgf(format string, v ...interface{}) {
	logString := fmt.Sprintf(format, v...)
	e.event.Msg(safeLogf("%s", logString))
}

func cleanString(text string) string {
	urlRegex := `[a-zA-Z][a-zA-Z0-9+.-]*:\/\/[a-zA-Z0-9+%/.\-:_?&=#@+]+`
	re := regexp.MustCompile(urlRegex)

	safeString := re.ReplaceAllString(text, "[redacted url]")
	return safeString
}

func safeLogf(format string, v ...any) string {
	safeLogs := os.Getenv("SAFE_LOGS") == "true"
	safeString := fmt.Sprintf(format, v...)
	if safeLogs {
		return cleanString(safeString)
	}
	return safeString
}

func (l *DefaultLogger) Log(format string) {
	l.logger.Info().Msg(safeLogf("%s", format))
}

func (l *DefaultLogger) Logf(format string, v ...any) {
	logString := fmt.Sprintf(format, v...)
	l.logger.Info().Msg(safeLogf("%s", logString))
}

func (l *DefaultLogger) Debug(format string) {
	debug := os.Getenv("DEBUG") == "true"

	if debug {
		l.logger.Debug().Msg(safeLogf("%s", format))
	}
}

func (l *DefaultLogger) Debugf(format string, v ...any) {
	debug := os.Getenv("DEBUG") == "true"
	logString := fmt.Sprintf(format, v...)

	if debug {
		l.logger.Debug().Msg(safeLogf("%s", logString))
	}
}

func (l *DefaultLogger) Error(format string) {
	l.logger.Error().Msg(safeLogf("%s", format))
}

func (l *DefaultLogger) Errorf(format string, v ...any) {
	logString := fmt.Sprintf(format, v...)
	l.logger.Error().Msg(safeLogf("%s", logString))
}

func (l *DefaultLogger) Warn(format string) {
	l.logger.Warn().Msg(safeLogf("%s", format))
}

func (l *DefaultLogger) Warnf(format string, v ...any) {
	logString := fmt.Sprintf(format, v...)
	l.logger.Warn().Msg(safeLogf("%s", logString))
}

func (l *DefaultLogger) Fatal(format string) {
	l.logger.Fatal().Msg(safeLogf("%s", format))
}

func (l *DefaultLogger) Fatalf(format string, v ...any) {
	logString := fmt.Sprintf(format, v...)
	l.logger.Fatal().Msg(safeLogf("%s", logString))
}

// InfoEvent implements the Logger interface for DefaultLogger
func (l *DefaultLogger) InfoEvent() Event {
	return &EventWrapper{event: l.logger.Info()}
}

// DebugEvent implements the Logger interface for DefaultLogger
func (l *DefaultLogger) DebugEvent() Event {
	debug := os.Getenv("DEBUG") == "true"
	if debug {
		return &EventWrapper{event: l.logger.Debug()}
	}
	// Return a no-op event when debug is disabled
	nopLogger := zerolog.Nop()
	return &EventWrapper{event: nopLogger.Debug()}
}

// WarnEvent implements the Logger interface for DefaultLogger
func (l *DefaultLogger) WarnEvent() Event {
	return &EventWrapper{event: l.logger.Warn()}
}

// ErrorEvent implements the Logger interface for DefaultLogger
func (l *DefaultLogger) ErrorEvent() Event {
	return &EventWrapper{event: l.logger.Error()}
}

// FatalEvent implements the Logger interface for DefaultLogger
func (l *DefaultLogger) FatalEvent() Event {
	return &EventWrapper{event: l.logger.Fatal()}
}
