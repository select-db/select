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
	"backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/cli"
	"backend/internal/middlewares"

	apikeyhandler "backend/internal/apikey"
	datasourcehandler "backend/internal/datasource"
	mcphandler "backend/internal/mcp"
	synchandler "backend/internal/syncer"
	workspacehandler "backend/internal/workspace"
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

	// Async writer + outbox drainer run until Stop on shutdown. Partitions are
	// managed in-DB by pg_partman + pg_cron.
	auditLogger := audit.New(db.GetDB(), audit.Options{})
	auditLogger.Start()
	audit.SetDefault(auditLogger)

	// pg_cron's scheduler lives in the cluster's cron DB, not the app DB, so the
	// maintenance job can't be a migration. No-op if AUDIT_CRON_DSN is unset,
	// then it's provisioned out of band (see the on-prem runbook).
	if cronDSN := os.Getenv("AUDIT_CRON_DSN"); cronDSN != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := audit.EnsureMaintenanceSchedule(ctx, db.GetDB(), cronDSN, os.Getenv("AUDIT_CRON_SCHEDULE")); err != nil {
			log.Printf("WARNING: audit: %v", err)
		}
		cancel()
	}

	// Warn loudly if partition maintenance can't run, so a misconfigured DB
	// surfaces now instead of silently stopping retention months later.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := audit.Preflight(ctx, db.GetDB()); err != nil {
			log.Printf("WARNING: %v", err)
		}
		cancel()
	}

	mux := http.NewServeMux()

	authenticated := middlewares.Authenticated()
	member := middlewares.Membership()
	// Per-endpoint rate limit (requests/minute, keyed by user else IP).
	// Applied innermost so authenticated routes key by user.
	limited := func(perMinute int, h http.HandlerFunc) http.Handler {
		return middlewares.RateLimit(perMinute)(h)
	}

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

	mux.Handle("/auth/get-device-code", limited(15, auth.GetDeviceCodeHandler()))
	mux.Handle("/auth/get-access-token", limited(30, auth.GetAccessTokenHandler()))

	mux.Handle("/sync/v1/sync", authenticated(limited(600, synchandler.Handler())))

	mux.Handle("/workspace/create", authenticated(limited(60, workspacehandler.CreateHandler())))
	mux.Handle("/workspace/delete", authenticated(member(limited(30, workspacehandler.DeleteHandler()))))

	mux.Handle("/user/search", authenticated(member(limited(120, workspacehandler.SearchUserHandler()))))
	mux.Handle("/user/add", authenticated(member(limited(120, workspacehandler.AddUserHandler()))))

	mux.Handle("/apikey/list", authenticated(member(limited(120, apikeyhandler.ListHandler()))))
	mux.Handle("/apikey/create", authenticated(member(limited(10, apikeyhandler.CreateHandler()))))
	mux.Handle("/apikey/revoke", authenticated(member(limited(30, apikeyhandler.RevokeHandler()))))
	mux.Handle("/apikey/rotate", authenticated(member(limited(10, apikeyhandler.RotateHandler()))))
	mux.Handle("/apikey/set-roles", authenticated(member(limited(30, apikeyhandler.SetRolesHandler()))))

	mux.Handle("/datasource/get", authenticated(member(limited(120, datasourcehandler.GetHandler()))))
	mux.Handle("/datasource/upsert", authenticated(member(limited(120, datasourcehandler.UpsertHandler()))))
	mux.Handle("/datasource/delete", authenticated(member(limited(60, datasourcehandler.DeleteHandler()))))

	mux.Handle("/datasource/ping", authenticated(member(limited(60, datasourcehandler.PingHandler()))))
	mux.Handle("/datasource/schema", authenticated(member(limited(120, datasourcehandler.SchemaHandler()))))
	mux.Handle("/datasource/execute", authenticated(member(limited(240, datasourcehandler.ExecuteHandler()))))
	mux.Handle("/datasource/dump", authenticated(member(limited(30, datasourcehandler.DumpHandler()))))

	mux.Handle("/mcp", authenticated(limited(600, mcphandler.Handler())))

	handler := middlewares.RequestLogger(mux)
	handler = middlewares.SecureHeaders(handler)

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

	// Drain after the server stops accepting requests, so no Log races the close.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer drainCancel()
	if err := auditLogger.Stop(drainCtx); err != nil {
		log.Printf("audit: shutdown drain: %v", err)
	}

	log.Println("Server stopped gracefully")
}
