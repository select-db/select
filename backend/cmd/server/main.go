package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/selectDb/dialect/engine"

	"backend/db"
	"backend/internal/cli"
	"backend/internal/httpapi"
	"backend/internal/middlewares"
)

// Version info, overridden at build time via:
//
//	go build -ldflags "-X main.version=1.2.3 -X main.minAppVersion=1.0.0 -X main.latestAppVersion=1.2.3" ./cmd/server
var (
	version          = "dev"
	minAppVersion    = "0.0.0"
	latestAppVersion = "0.0.0"
)

// versionHandler reports the running backend version and where clients should
// fetch release assets. release_base_url is composed from RELEASE_BASE_URL (the
// releases-download root) plus the tag; empty means the client falls back to
// its built-in location.
func versionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		releaseBaseURL := ""
		if base := os.Getenv("RELEASE_BASE_URL"); base != "" {
			releaseBaseURL = strings.TrimRight(base, "/") + "/v" + latestAppVersion
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"backend":            version,
			"min_app_version":    minAppVersion,
			"latest_app_version": latestAppVersion,
			"release_base_url":   releaseBaseURL,
		})
	}
}

func main() {
	// Best-effort: load .env if present. In production, vars come from systemd EnvironmentFile.
	_ = godotenv.Load()

	// This process dials user-supplied datasources on behalf of other users,
	// so enforce the SSRF guard. The desktop app must never set this.
	engine.EnforceOutboundGuard = true

	if err := db.Init(); err != nil {
		log.Fatalf("DB init failed: %v", err)
	}

	if len(os.Args) > 1 {
		cmd := &cli.Command{}
		var arg string
		if len(os.Args) > 2 {
			arg = os.Args[2]
		}
		output, err := cmd.Run(os.Args[1], arg)
		fmt.Print(output)
		if err != nil {
			log.Fatalf("Error running command: %v", err)
		}
		return
	}

	startPprofServer()

	auditLogger := startAuditLogger()

	mux := http.NewServeMux()

	// Per-endpoint rate limit (requests/minute, keyed by user else IP).
	limited := func(perMinute int, h http.HandlerFunc) http.Handler {
		return middlewares.RateLimit(perMinute)(h)
	}

	// Infrastructure routes carry build-time vars (version) and process-local
	// checks (health), so they stay here; the application routes live in httpapi
	// so the e2e harness builds the identical handler graph.
	mux.Handle("/health", limited(60, func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			log.Printf("health: db ping: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	mux.Handle("/version", limited(60, versionHandler()))

	httpapi.Register(mux)
	handler := httpapi.Wrap(mux)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
		// Slowloris guard. No Read/WriteTimeout: execute/dump stream large
		// responses and would be truncated by a global write deadline.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Run server in goroutine so we can listen for shutdown signals
	go func() {
		log.Println("Server listening on http://localhost:8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	// Setup channel to listen for interrupt or terminate signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // wait here until signal received
	log.Println("Shutting down server...")

	// Create a deadline to wait for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	stopAuditLogger(auditLogger)

	log.Println("Server stopped gracefully")
}
