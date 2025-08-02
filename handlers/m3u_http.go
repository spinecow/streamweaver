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
	logger.InfoEvent().
		Str("component", "M3UHTTPHandler").
		Str("processed_path", processedPath).
		Msg("Creating new M3UHTTPHandler")
	h := &M3UHTTPHandler{
		logger:        logger,
		processedPath: processedPath,
		credentials:   make(map[string]Credential),
	}
	h.loadCredentials()
	logger.InfoEvent().
		Str("component", "M3UHTTPHandler").
		Int("credentials_count", len(h.credentials)).
		Msg("M3UHTTPHandler created")
	return h
}

func (h *M3UHTTPHandler) loadCredentials() {
	// Note: This method is called during initialization, so we can't add correlation ID yet
	credsStr := os.Getenv("CREDENTIALS")
	h.logger.Debugf("Loading credentials from environment variable. Length: %d", len(credsStr))

	if credsStr == "" || strings.ToLower(credsStr) == "none" {
		h.logger.InfoEvent().
			Str("component", "M3UHTTPHandler").
			Msg("No credentials configured, authentication disabled")
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

	h.logger.InfoEvent().
		Str("component", "M3UHTTPHandler").
		Int("credentials_count", len(creds)).
		Msg("Successfully parsed credentials from environment")
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

	h.logger.InfoEvent().
		Str("component", "M3UHTTPHandler").
		Int("credentials_count", len(h.credentials)).
		Msg("Loaded credentials into memory")
}

func (h *M3UHTTPHandler) SetProcessedPath(path string) {
	h.processedPath = path
}

func (h *M3UHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// Create a logger with correlation ID
	requestLogger := h.logger.WithCorrelationID(r.Context())

	requestLogger.DebugEvent().
		Str("component", "M3UHTTPHandler").
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Str("client_ip", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Str("url_path", r.URL.Path).
		Msg("M3U playlist request received")

	w.Header().Set("Access-control-allow-origin", "*")
	isAuthorized := h.handleAuthWithLogger(r, requestLogger)
	if !isAuthorized {
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Str("client_ip", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Int("status_code", http.StatusForbidden).
			Dur("duration", time.Since(startTime)).
			Msg("Unauthorized access attempt")
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	if h.processedPath == "" {
		requestLogger.ErrorEvent().
			Str("component", "M3UHTTPHandler").
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Str("client_ip", r.RemoteAddr).
			Int("status_code", http.StatusNotFound).
			Dur("duration", time.Since(startTime)).
			Msg("No processed M3U found")
		http.Error(w, "No processed M3U found.", http.StatusNotFound)
		return
	}
	
	requestLogger.InfoEvent().
		Str("component", "M3UHTTPHandler").
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Str("client_ip", r.RemoteAddr).
		Str("playlist_url", h.processedPath).
		Int("status_code", http.StatusOK).
		Dur("duration", time.Since(startTime)).
		Msg("Serving M3U playlist")
	http.ServeFile(w, r, h.processedPath)
}

func (h *M3UHTTPHandler) handleAuthWithLogger(r *http.Request, requestLogger logger.Logger) bool {
	requestLogger.DebugEvent().
		Str("component", "M3UHTTPHandler").
		Str("client_ip", r.RemoteAddr).
		Int("credentials_count", len(h.credentials)).
		Msg("Handling authentication request")

	if len(h.credentials) == 0 {
		requestLogger.DebugEvent().
			Str("component", "M3UHTTPHandler").
			Str("client_ip", r.RemoteAddr).
			Msg("No credentials loaded, allowing access")
		return true // No authentication required
	}

	user, pass := r.URL.Query().Get("username"), r.URL.Query().Get("password")
	requestLogger.DebugEvent().
		Str("component", "M3UHTTPHandler").
		Str("client_ip", r.RemoteAddr).
		Str("username", user).
		Msg("Authentication attempt")

	if user == "" || pass == "" {
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("client_ip", r.RemoteAddr).
			Str("username", user).
			Bool("username_provided", user != "").
			Bool("password_provided", pass != "").
			Msg("Missing username or password in request")
		return false
	}

	cred, ok := h.credentials[strings.ToLower(user)]
	if !ok {
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("client_ip", r.RemoteAddr).
			Str("username", user).
			Msg("User not found in credentials")
		return false
	}

	// Constant-time comparison for passwords
	if subtle.ConstantTimeCompare([]byte(pass), []byte(cred.Password)) != 1 {
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("client_ip", r.RemoteAddr).
			Str("username", user).
			Msg("Password mismatch for user")
		return false
	}

	// Check expiration
	if !cred.Expiration.IsZero() && time.Now().After(cred.Expiration) {
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("client_ip", r.RemoteAddr).
			Str("username", user).
			Time("expiration", cred.Expiration).
			Msg("Credential expired for user")
		return false
	}

	requestLogger.InfoEvent().
		Str("component", "M3UHTTPHandler").
		Str("client_ip", r.RemoteAddr).
		Str("username", user).
		Msg("Authentication successful")
	return true
}
