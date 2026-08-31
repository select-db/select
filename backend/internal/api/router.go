// Package api holds the app's HTTP route table and middleware chain, factored
// out of cmd/server/main.go so both the server binary and end-to-end tests build
// the exact same handler graph (same auth/membership/rate-limit wrapping). main
// keeps its build-var routes (/version, /health) and calls Register for the rest.
package api

import (
	"net/http"

	"backend/internal/middlewares"

	"backend/internal/api/gen"
	apikeyhandler "backend/internal/apikey"
	"backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/authz"
	datasourcehandler "backend/internal/datasource"
	mcphandler "backend/internal/mcp"
	synchandler "backend/internal/syncer"
	workspacehandler "backend/internal/workspace"
)

// Register installs the application routes on mux with the production middleware
// chain. Callers add their own infrastructure routes (health, version) before or
// after, then wrap the mux with Wrap.
func Register(mux *http.ServeMux) {
	// Workspace scoping: reads the X-Workspace-Id header (so GET routes work) or
	// a workspace_id in the body, validated against the caller's memberships.
	member := middlewares.Membership()
	// Per-endpoint rate limit (requests/minute, keyed by user else IP).
	// Applied innermost so authenticated routes key by user.
	limited := func(perMinute int, h http.HandlerFunc) http.Handler {
		return middlewares.RateLimit(perMinute)(h)
	}

	// authenticated validates the token and then stashes an audit principal
	// resolver on the context, so any handler below can attach the actor to an
	// audit event via audit.ResolvePrincipal without threading it through
	// signatures. (The resolver lives here, not in middlewares, because it needs
	// authz — which imports middlewares.)
	authMW := middlewares.Authenticated()
	authenticated := func(h http.Handler) http.Handler {
		return authMW(withPrincipalResolver(h))
	}

	mux.Handle("GET /auth/device-code", limited(15, auth.GetDeviceCodeHandler()))
	mux.Handle("POST /auth/access-token", limited(30, auth.GetAccessTokenHandler()))

	mux.Handle("POST /sync/v1/sync", authenticated(limited(600, synchandler.Handler())))
	mux.Handle("POST /mcp", authenticated(limited(600, mcphandler.Handler())))

	mux.Handle("POST /workspaces", authenticated(limited(60, workspacehandler.CreateHandler())))
	mux.Handle("DELETE /workspaces/{id}", authenticated(member(limited(30, workspacehandler.DeleteHandler()))))

	// The only write path for workspace.logo: it re-encodes the image, which the
	// sync commit path cannot do.
	mux.Handle("PUT /workspaces/{id}/logo", authenticated(workspacehandler.LimitLogoBody(member(limited(10, workspacehandler.UpdateLogoHandler())))))

	mux.Handle("GET /users/search", authenticated(member(limited(120, workspacehandler.SearchUserHandler()))))
	mux.Handle("POST /users", authenticated(member(limited(120, workspacehandler.AddUserHandler()))))

	mux.Handle("GET /apikeys", authenticated(member(limited(120, apikeyhandler.ListHandler()))))
	mux.Handle("POST /apikeys", authenticated(member(limited(10, apikeyhandler.CreateHandler()))))
	mux.Handle("POST /apikeys/{id}/revoke", authenticated(member(limited(30, apikeyhandler.RevokeHandler()))))
	mux.Handle("POST /apikeys/{id}/rotate", authenticated(member(limited(10, apikeyhandler.RotateHandler()))))
	mux.Handle("PUT /apikeys/{id}/roles", authenticated(member(limited(30, apikeyhandler.SetRolesHandler()))))

	mux.Handle("GET /datasources/{id}", authenticated(member(limited(120, datasourcehandler.GetHandler()))))
	mux.Handle("PUT /datasources/{id}", authenticated(member(limited(120, datasourcehandler.UpsertHandler()))))
	mux.Handle("DELETE /datasources/{id}", authenticated(member(limited(60, datasourcehandler.DeleteHandler()))))

	mux.Handle("POST /datasources/{id}/ping", authenticated(member(limited(60, datasourcehandler.PingHandler()))))
	mux.Handle("GET /datasources/{id}/schema", authenticated(member(limited(120, datasourcehandler.SchemaHandler()))))
	mux.Handle("POST /datasources/{id}/execute", authenticated(member(limited(240, datasourcehandler.ExecuteHandler()))))
	mux.Handle("GET /datasources/{id}/dump", authenticated(member(limited(30, datasourcehandler.DumpHandler()))))

	// Generated REST API (roles, permissions, groups, junctions, audit log).
	// Shares the same auth + workspace-scoping + rate-limit chain; per-op
	// required actions are enforced inside each handler.
	gen.RegisterRoutes(mux, func(perMinute int, h http.Handler) http.Handler {
		return authenticated(member(middlewares.RateLimit(perMinute)(h)))
	})
}

// withPrincipalResolver stashes an audit principal resolver on the request
// context. It runs inside authenticated, so the principal it reads is already
// populated; handlers call audit.ResolvePrincipal(ctx, workspaceID) to attach the
// actor. The workspace is supplied per event because a request can act across
// workspaces (the sync path).
func withPrincipalResolver(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := audit.ContextWithPrincipalResolver(r.Context(), func(workspaceID string) audit.Principal {
			return authz.RequestPrincipal(r, workspaceID)
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Wrap applies the outer middleware (request logging, secure headers) shared by
// the server and the e2e harness.
func Wrap(mux *http.ServeMux) http.Handler {
	handler := middlewares.RequestLogger(mux)
	handler = middlewares.SecureHeaders(handler)
	return handler
}
