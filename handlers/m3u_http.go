package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"m3u-stream-merger/logger"
)

type Credential struct {
	Username   string    `json:"username"`
	Password   string    `json:"password"`
	Expiration time.Time `json:"expiration"`
}

type M3UHTTPHandler struct {
	logger        logger.Logger
	processedPath string
	credentials   map[string]Credential
}

func NewM3UHTTPHandler(logger logger.Logger, processedPath string) *M3UHTTPHandler {
	// Note: This logger is used during initialization, so we can't add correlation ID yet
	logger.Logf("Creating new M3UHTTPHandler with processedPath: %s", processedPath)
	h := &M3UHTTPHandler{
		logger:        logger,
		processedPath: processedPath,
		credentials:   make(map[string]Credential),
	}
	h.loadCredentials()
	logger.Logf("M3UHTTPHandler created with %d credentials loaded", len(h.credentials))
	return h
}

func (h *M3UHTTPHandler) loadCredentials() {
	// Note: This method is called during initialization, so we can't add correlation ID yet
	credsStr := os.Getenv("CREDENTIALS")
	h.logger.Debugf("Loading credentials from environment variable. Length: %d", len(credsStr))

	if credsStr == "" || strings.ToLower(credsStr) == "none" {
		h.logger.Log("No credentials configured, authentication disabled")
		return
	}

	var creds []Credential
	err := json.Unmarshal([]byte(credsStr), &creds)
	if err != nil {
		h.logger.ErrorEvent().
			Str("component", "M3UHTTPHandler").
			Err(err).
			Msg("Error parsing credentials")
		return
	}

	h.logger.Logf("Successfully parsed %d credentials from environment", len(creds))
	for i, cred := range creds {
		// Log credential info without sensitive data
		if cred.Expiration.IsZero() {
			h.logger.Debugf("Credential %d: username=%s, password=(redacted), expiration=none",
				i+1, cred.Username)
		} else {
			h.logger.Debugf("Credential %d: username=%s, password=(redacted), expiration=%s",
				i+1, cred.Username, cred.Expiration.Format(time.RFC3339))
		}
		h.credentials[strings.ToLower(cred.Username)] = cred
	}

	h.logger.Logf("Loaded %d credentials into memory", len(h.credentials))
}

func (h *M3UHTTPHandler) SetProcessedPath(path string) {
	h.processedPath = path
}

func (h *M3UHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a logger with correlation ID
	requestLogger := h.logger.WithCorrelationID(r.Context())

	requestLogger.Debugf("ServeHTTP called with path: %s", r.URL.Path)

	w.Header().Set("Access-control-allow-origin", "*")
	isAuthorized := h.handleAuthWithLogger(r, requestLogger)
	if !isAuthorized {
		requestLogger.Logf("Unauthorized access attempt to %s", r.URL.Path)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	if h.processedPath == "" {
		requestLogger.ErrorEvent().
			Str("component", "M3UHTTPHandler").
			Msg("No processed M3U found")
		http.Error(w, "No processed M3U found.", http.StatusNotFound)
		return
	}
	requestLogger.Debugf("Serving file: %s", h.processedPath)
	http.ServeFile(w, r, h.processedPath)
}

func (h *M3UHTTPHandler) handleAuthWithLogger(r *http.Request, requestLogger logger.Logger) bool {
	requestLogger.Debugf("Handling authentication request. Number of loaded credentials: %d", len(h.credentials))

	if len(h.credentials) == 0 {
		requestLogger.Debug("No credentials loaded, allowing access")
		return true // No authentication required
	}

	user, pass := r.URL.Query().Get("username"), r.URL.Query().Get("password")
	requestLogger.Debugf("Authentication attempt with username: %s, password: (redacted)", user)

	if user == "" || pass == "" {
		requestLogger.Debug("Missing username or password in request")
		return false
	}

	cred, ok := h.credentials[strings.ToLower(user)]
	if !ok {
		requestLogger.Debugf("User %s not found in credentials", user)
		return false
	}

	// Constant-time comparison for passwords
	if subtle.ConstantTimeCompare([]byte(pass), []byte(cred.Password)) != 1 {
		requestLogger.Debug("Password mismatch for user")
		return false
	}

	// Check expiration
	if !cred.Expiration.IsZero() && time.Now().After(cred.Expiration) {
		requestLogger.Debugf("Credential expired for user: %s", user)
		return false
	}

	requestLogger.Debug("Authentication successful")
	return true
}
