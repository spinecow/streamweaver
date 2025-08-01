package logger

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/rs/zerolog"
)

type DefaultLogger struct {
	Logger
}

var Default = &DefaultLogger{}

var logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()

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

func (*DefaultLogger) Log(format string) {
	logger.Info().Msg(safeLogf("%s", format))
}

func (*DefaultLogger) Logf(format string, v ...any) {
	logString := fmt.Sprintf(format, v...)
	logger.Info().Msg(safeLogf("%s", logString))
}

func (*DefaultLogger) Debug(format string) {
	debug := os.Getenv("DEBUG") == "true"

	if debug {
		logger.Debug().Msg(safeLogf("%s", format))
	}
}

func (*DefaultLogger) Debugf(format string, v ...any) {
	debug := os.Getenv("DEBUG") == "true"
	logString := fmt.Sprintf(format, v...)

	if debug {
		logger.Debug().Msg(safeLogf("%s", logString))
	}
}

func (*DefaultLogger) Error(format string) {
	logger.Error().Msg(safeLogf("%s", format))
}

func (*DefaultLogger) Errorf(format string, v ...any) {
	logString := fmt.Sprintf(format, v...)
	logger.Error().Msg(safeLogf("%s", logString))
}

func (*DefaultLogger) Warn(format string) {
	logger.Warn().Msg(safeLogf("%s", format))
}

func (*DefaultLogger) Warnf(format string, v ...any) {
	logString := fmt.Sprintf(format, v...)
	logger.Warn().Msg(safeLogf("%s", logString))
}

func (*DefaultLogger) Fatal(format string) {
	logger.Fatal().Msg(safeLogf("%s", format))
}

func (*DefaultLogger) Fatalf(format string, v ...any) {
	logString := fmt.Sprintf(format, v...)
	logger.Fatal().Msg(safeLogf("%s", logString))
}

// InfoEvent implements the Logger interface for DefaultLogger
func (*DefaultLogger) InfoEvent() Event {
	return &EventWrapper{event: logger.Info()}
}

// DebugEvent implements the Logger interface for DefaultLogger
func (*DefaultLogger) DebugEvent() Event {
	debug := os.Getenv("DEBUG") == "true"
	if debug {
		return &EventWrapper{event: logger.Debug()}
	}
	// Return a no-op event when debug is disabled
	nopLogger := zerolog.Nop()
	return &EventWrapper{event: nopLogger.Debug()}
}

// WarnEvent implements the Logger interface for DefaultLogger
func (*DefaultLogger) WarnEvent() Event {
	return &EventWrapper{event: logger.Warn()}
}

// ErrorEvent implements the Logger interface for DefaultLogger
func (*DefaultLogger) ErrorEvent() Event {
	return &EventWrapper{event: logger.Error()}
}

// FatalEvent implements the Logger interface for DefaultLogger
func (*DefaultLogger) FatalEvent() Event {
	return &EventWrapper{event: logger.Fatal()}
}
