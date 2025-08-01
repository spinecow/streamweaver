package correlation

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// Context key for correlation ID
type contextKey string

const (
	// CorrelationIDKey is the context key for storing the correlation ID
	CorrelationIDKey contextKey = "correlation_id"
)

// GenerateCorrelationID generates a new UUID for use as a correlation ID
func GenerateCorrelationID() string {
	return uuid.New().String()
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, id)
}

// CorrelationIDFromContext retrieves the correlation ID from the context
func CorrelationIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(CorrelationIDKey).(string)
	return id, ok
}

// CorrelationIDFromRequest retrieves the correlation ID from the request context
func CorrelationIDFromRequest(r *http.Request) (string, bool) {
	return CorrelationIDFromContext(r.Context())
}