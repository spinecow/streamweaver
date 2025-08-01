package correlation

import (
	"net/http"
)

// Middleware is HTTP middleware that generates a correlation ID for each request
// and adds it to the request context
func Middleware(next http.Handler) http.Handler {
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