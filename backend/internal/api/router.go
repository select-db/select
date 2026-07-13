// Package api holds the app's HTTP route table and middleware chain, factored
// out of cmd/server/main.go so both the server binary and end-to-end tests build
// the exact same handler graph (same auth/membership/rate-limit wrapping). main
// keeps its build-var routes (/version, /health) and calls Register for the rest.
package api

import (
	"net/http"

	"backend/internal/middlewares"

	apikeyhandler "backend/internal/apikey"
	"backend/internal/auth"
	datasourcehandler "backend/internal/datasource"
	mcphandler "backend/internal/mcp"
	synchandler "backend/internal/syncer"
	workspacehandler "backend/internal/workspace"
)

// Register installs the application routes on mux with the production middleware
// chain. Callers add their own infrastructure routes (health, version) before or
// after, then wrap the mux with Wrap.
func Register(mux *http.ServeMux) {
	authenticated := middlewares.Authenticated()
	member := middlewares.Membership()
	// Per-endpoint rate limit (requests/minute, keyed by user else IP).
	// Applied innermost so authenticated routes key by user.
	limited := func(perMinute int, h http.HandlerFunc) http.Handler {
		return middlewares.RateLimit(perMinute)(h)
	}

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
}

// Wrap applies the outer middleware (request logging, secure headers) shared by
// the server and the e2e harness.
func Wrap(mux *http.ServeMux) http.Handler {
	handler := middlewares.RequestLogger(mux)
	handler = middlewares.SecureHeaders(handler)
	return handler
}
