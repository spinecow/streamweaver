package main

import (
	"context"
	"fmt"
	"m3u-stream-merger/correlation"
	"m3u-stream-merger/handlers"
	"m3u-stream-merger/logger"
	"m3u-stream-merger/updater"
	"net/http"
	"os"
	"time"
)

func main() {
	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m3uHandler := handlers.NewM3UHTTPHandler(logger.Default, "")
	streamHandler := handlers.NewStreamHTTPHandler(handlers.NewDefaultProxyInstance(), logger.Default)
	passthroughHandler := handlers.NewPassthroughHTTPHandler(logger.Default)

	logger.Default.InfoEvent().
		Str("component", "main").
		Msg("Starting updater")
	_, err := updater.Initialize(ctx, logger.Default, m3uHandler)
	if err != nil {
		logger.Default.Fatalf("Error initializing updater: %v", err)
	}

	// manually set time zone
	if tz := os.Getenv("TZ"); tz != "" {
		var err error
		time.Local, err = time.LoadLocation(tz)
		if err != nil {
			logger.Default.Fatalf("error loading location '%s': %v\n", tz, err)
		}
	}

	logger.Default.InfoEvent().
		Str("component", "main").
		Msg("Setting up HTTP handlers")
	// HTTP handlers with correlation ID middleware
	http.Handle("/playlist.m3u", correlation.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m3uHandler.ServeHTTP(w, r)
	})))
	http.Handle("/p/", correlation.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamHandler.ServeHTTP(w, r)
	})))
	http.Handle("/a/", correlation.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		passthroughHandler.ServeHTTP(w, r)
	})))
	http.Handle("/segment/", correlation.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamHandler.ServeSegmentHTTP(w, r)
	})))

	// Start the server
	logger.Default.InfoEvent().
		Str("component", "main").
		Str("port", os.Getenv("PORT")).
		Msg("Server is running")
	logger.Default.InfoEvent().
		Str("component", "main").
		Str("endpoint", "/playlist.m3u").
		Msg("Playlist Endpoint is running")
	logger.Default.InfoEvent().
		Str("component", "main").
		Str("endpoint", "/p/{originalBasePath}/{streamID}.{fileExt}").
		Msg("Stream Endpoint is running")
	err = http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil)
	if err != nil {
		logger.Default.Fatalf("HTTP server error: %v", err)
	}
}
