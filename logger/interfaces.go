package logger

import (
	"context"
	"time"
)

type Logger interface {
	Log(format string)
	Logf(format string, v ...any)

	Warn(format string)
	Warnf(format string, v ...any)

	Debug(format string)
	Debugf(format string, v ...any)

	Error(format string)
	Errorf(format string, v ...any)

	Fatal(format string)
	Fatalf(format string, v ...any)

	// Structured logging methods
	WithFields(fields map[string]interface{}) Logger
	WithSensitiveField(key, value string) Logger
	WithCorrelationID(ctx context.Context) Logger
	InfoEvent() Event
	DebugEvent() Event
	WarnEvent() Event
	ErrorEvent() Event
	FatalEvent() Event
}

type Event interface {
	Str(key, val string) Event
	Int(key string, i int) Event
	Int64(key string, i int64) Event
	Float64(key string, f float64) Event
	Bool(key string, b bool) Event
	Time(key string, t time.Time) Event
	Dur(key string, d time.Duration) Event
	Err(err error) Event
	Msg(msg string)
	Msgf(format string, v ...interface{})
}
