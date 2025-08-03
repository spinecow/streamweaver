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
	h.logger.DebugEvent().
		Str("component", "M3UHTTPHandler").
		Int("credentials_length", len(credsStr)).
		Msg("Loading credentials from environment variable")

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
			h.logger.DebugEvent().
				Str("component", "M3UHTTPHandler").
				Int("credential_index", i+1).
				Str("username", cred.Username).
				Msg("Credential loaded with no expiration")
		} else {
			h.logger.DebugEvent().
				Str("component", "M3UHTTPHandler").
				Int("credential_index", i+1).
				Str("username", cred.Username).
				Time("expiration", cred.Expiration).
				Msg("Credential loaded with expiration")
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
	authStartTime := time.Now()
	
	requestLogger.DebugEvent().
		Str("component", "M3UHTTPHandler").
		Str("operation", "authenticate").
		Str("client_ip", r.RemoteAddr).
		Int("credentials_count", len(h.credentials)).
		Msg("Handling authentication request")

	if len(h.credentials) == 0 {
		authDuration := time.Since(authStartTime)
		requestLogger.DebugEvent().
			Str("component", "M3UHTTPHandler").
			Str("operation", "authenticate").
			Str("client_ip", r.RemoteAddr).
			Dur("auth_duration", authDuration).
			Msg("No credentials loaded, allowing access")
		return true // No authentication required
	}

	// Measure parameter extraction time
	paramStartTime := time.Now()
	user, pass := r.URL.Query().Get("username"), r.URL.Query().Get("password")
	paramDuration := time.Since(paramStartTime)
	
	requestLogger.DebugEvent().
		Str("component", "M3UHTTPHandler").
		Str("operation", "extract_auth_params").
		Str("client_ip", r.RemoteAddr).
		Str("username", user).
		Dur("param_duration", paramDuration).
		Msg("Authentication attempt")

	if user == "" || pass == "" {
		authDuration := time.Since(authStartTime)
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("operation", "authenticate").
			Str("client_ip", r.RemoteAddr).
			Str("username", user).
			Bool("username_provided", user != "").
			Bool("password_provided", pass != "").
			Dur("auth_duration", authDuration).
			Msg("Missing username or password in request")
		return false
	}

	// Measure credential lookup time
	lookupStartTime := time.Now()
	cred, ok := h.credentials[strings.ToLower(user)]
	lookupDuration := time.Since(lookupStartTime)
	
	if !ok {
		authDuration := time.Since(authStartTime)
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("operation", "authenticate").
			Str("client_ip", r.RemoteAddr).
			Str("username", user).
			Dur("lookup_duration", lookupDuration).
			Dur("auth_duration", authDuration).
			Msg("User not found in credentials")
		return false
	}

	// Measure password comparison time
	compareStartTime := time.Now()
	passwordMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(cred.Password)) == 1
	compareDuration := time.Since(compareStartTime)
	
	if !passwordMatch {
		authDuration := time.Since(authStartTime)
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("operation", "authenticate").
			Str("client_ip", r.RemoteAddr).
			Str("username", user).
			Dur("lookup_duration", lookupDuration).
			Dur("compare_duration", compareDuration).
			Dur("auth_duration", authDuration).
			Msg("Password mismatch for user")
		return false
	}

	// Measure expiration check time
	expirationStartTime := time.Now()
	isExpired := !cred.Expiration.IsZero() && time.Now().After(cred.Expiration)
	expirationDuration := time.Since(expirationStartTime)
	
	if isExpired {
		authDuration := time.Since(authStartTime)
		requestLogger.WarnEvent().
			Str("component", "M3UHTTPHandler").
			Str("operation", "authenticate").
			Str("client_ip", r.RemoteAddr).
			Str("username", user).
			Time("expiration", cred.Expiration).
			Dur("lookup_duration", lookupDuration).
			Dur("compare_duration", compareDuration).
			Dur("expiration_duration", expirationDuration).
			Dur("auth_duration", authDuration).
			Msg("Credential expired for user")
		return false
	}

	authDuration := time.Since(authStartTime)
	requestLogger.InfoEvent().
		Str("component", "M3UHTTPHandler").
		Str("operation", "authenticate").
		Str("client_ip", r.RemoteAddr).
		Str("username", user).
		Dur("lookup_duration", lookupDuration).
		Dur("compare_duration", compareDuration).
		Dur("expiration_duration", expirationDuration).
		Dur("param_duration", paramDuration).
		Dur("auth_duration", authDuration).
		Msg("Authentication successful")
	return true
}
