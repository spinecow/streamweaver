package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"m3u-stream-merger/logger"
	"net/http"
	"strings"
)

func GenerateFingerprintWithContext(ctx context.Context, r *http.Request) string {
	// Create a logger with correlation ID
	requestLogger := logger.Default.WithCorrelationID(ctx)

	// Collect relevant attributes
	ip := strings.Split(r.RemoteAddr, ":")[0]
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = xff
	}
	userAgent := r.Header.Get("User-Agent")
	accept := r.Header.Get("Accept")
	acceptLang := r.Header.Get("Accept-Language")
	path := r.URL.Path

	// Combine into a single string
	data := fmt.Sprintf("%s|%s|%s|%s|%s", ip, userAgent, accept, acceptLang, path)
	requestLogger.Debugf("Generating fingerprint from: %s", data)

	// Hash the string for a compact, fixed-length identifier
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func GenerateFingerprint(r *http.Request) string {
	return GenerateFingerprintWithContext(r.Context(), r)
}
