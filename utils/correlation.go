package utils

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

// CorrelationIDMiddleware is HTTP middleware that generates a correlation ID for each request
// and adds it to the request context
func CorrelationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if correlation ID already exists in request headers
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			// Generate a new correlation ID if one doesn't exist
			correlationID = GenerateCorrelationID()
		}

		// Add correlation ID to request context
		ctx := WithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)

		// Add correlation ID to response headers for traceability
		w.Header().Set("X-Correlation-ID", correlationID)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}