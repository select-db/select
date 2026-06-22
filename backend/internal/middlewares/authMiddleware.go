package middlewares

import (
	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	auth "backend/internal/auth"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	userIDKey          = contextKey("user_id")
	principalNameKey   = contextKey("principal_name")
	workspacesKey      = contextKey("workspaces")
	apiKeyPrincipalKey = contextKey("api_key_principal")
)

// IsAPIKeyPrincipal reports whether the request authenticated via API key
func IsAPIKeyPrincipal(r *http.Request) bool {
	v, _ := r.Context().Value(apiKeyPrincipalKey).(bool)
	return v
}

func MustGetUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized: missing user context", http.StatusUnauthorized)
		return "", false
	}
	return userID, true
}

// GetWorkspaces returns the caller's per-workspace standing (membership +
// ownership + roles) — the single structure every workspace getter derives from.
func GetWorkspaces(r *http.Request) ([]auth.WorkspaceClaim, bool) {
	ws, ok := r.Context().Value(workspacesKey).([]auth.WorkspaceClaim)
	return ws, ok
}

func workspaceIDsOf(ws []auth.WorkspaceClaim) []string {
	ids := make([]string, 0, len(ws))
	for _, w := range ws {
		ids = append(ids, w.ID)
	}
	return ids
}

func MustGetWorkspaceIDs(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	// Fail closed: buildAuthContext sets this on success; absence = derivation
	// failed, don't proceed with an implicit empty set.
	ws, ok := GetWorkspaces(r)
	if !ok {
		http.Error(w, "Unauthorized: missing workspace context", http.StatusUnauthorized)
		return nil, false
	}
	return workspaceIDsOf(ws), true
}

func GetWorkspaceIDs(r *http.Request) ([]string, bool) {
	ws, ok := GetWorkspaces(r)
	if !ok {
		return nil, false
	}
	return workspaceIDsOf(ws), true
}

func GetOwnedWorkspaceIDs(r *http.Request) []string {
	ws, _ := GetWorkspaces(r)
	var owned []string
	for _, w := range ws {
		if w.IsOwner {
			owned = append(owned, w.ID)
		}
	}
	return owned
}

// Principal is the calling principal's request-known identity: who they are and
// their per-workspace standing, resolved from the JWT or API-key auth.
// Permissions live in the authz layer; read this with GetPrincipal.
type Principal struct {
	ID         string
	Name       string
	IsAPIKey   bool
	Workspaces []auth.WorkspaceClaim
}

// Workspace returns the caller's standing in workspaceID (roles, ownership).
func (p Principal) Workspace(workspaceID string) (auth.WorkspaceClaim, bool) {
	for _, w := range p.Workspaces {
		if w.ID == workspaceID {
			return w, true
		}
	}
	return auth.WorkspaceClaim{}, false
}

// GetPrincipal returns the caller's identity in one read. Callers take the
// fields they care about instead of calling several small getters.
func GetPrincipal(r *http.Request) Principal {
	ctx := r.Context()
	id, _ := ctx.Value(userIDKey).(string)
	name, _ := ctx.Value(principalNameKey).(string)
	ws, _ := ctx.Value(workspacesKey).([]auth.WorkspaceClaim)
	return Principal{
		ID:         id,
		Name:       name,
		IsAPIKey:   IsAPIKeyPrincipal(r),
		Workspaces: ws,
	}
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// buildAuthContext attaches the userID, name, and the per-workspace standing to
// the context. Membership is re-derived from the DB for tenant isolation (not
// trusted from the token); each member workspace's roles/ownership are attached
// from the token's claim. Permissions are resolved on demand by the authz layer.
func buildAuthContext(ctx context.Context, userID, name string, claimWorkspaces []auth.WorkspaceClaim) (context.Context, error) {
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, principalNameKey, name)

	if db.Queries == nil {
		return ctx, errors.New("workspace lookup unavailable")
	}
	uid, err := db_types.NewJSONNullUUIDFromString(userID)
	if err != nil {
		return ctx, fmt.Errorf("invalid user id: %w", err)
	}
	ids, err := db.Queries.GetWorkspaceIDsByUserID(ctx, uid)
	if err != nil {
		return ctx, fmt.Errorf("workspace lookup failed: %w", err)
	}

	claimByWS := make(map[string]auth.WorkspaceClaim, len(claimWorkspaces))
	for _, w := range claimWorkspaces {
		claimByWS[w.ID] = w
	}
	workspaces := make([]auth.WorkspaceClaim, 0, len(ids))
	for _, u := range ids {
		if !u.Valid {
			continue
		}
		id := u.UUID.String()
		c := claimByWS[id] // zero value if the token predates this membership
		workspaces = append(workspaces, auth.WorkspaceClaim{ID: id, IsOwner: c.IsOwner, Roles: c.Roles})
	}
	return context.WithValue(ctx, workspacesKey, workspaces), nil
}

// buildAPIKeyContext resolves an API key to the same principal context a JWT
// builds. Workspace comes from the key, so API-key routes skip Membership()
func buildAPIKeyContext(ctx context.Context, token string) (context.Context, error) {
	if db.Queries == nil {
		return ctx, errors.New("api key lookup unavailable")
	}
	prefix, ok := auth.ParseAPIKeyPrefix(token)
	if !ok {
		return ctx, errors.New("malformed api key")
	}
	row, err := db.Queries.GetAPIKeyByPrefix(ctx, db_types.NewJSONNullString(prefix))
	if err != nil {
		return ctx, errors.New("unknown api key")
	}
	// verify before trusting any field on the row
	if !auth.VerifyAPIKey(token, row.HashedKey.ValueOrEmpty()) {
		return ctx, errors.New("api key mismatch")
	}
	if row.ExpiresAt.Valid && row.ExpiresAt.Time.Before(time.Now()) {
		return ctx, errors.New("api key expired")
	}
	if !row.WorkspaceID.Valid {
		return ctx, errors.New("api key has no workspace")
	}

	roleRows, err := db.Queries.GetAPIKeyRolesWithNames(ctx, row.ID)
	if err != nil {
		return ctx, fmt.Errorf("api key role lookup failed: %w", err)
	}
	roles := make([]auth.RoleRef, 0, len(roleRows))
	for _, rr := range roleRows {
		if rr.ID.Valid {
			roles = append(roles, auth.RoleRef{ID: rr.ID.UUID.String(), Name: rr.Name.ValueOrEmpty()})
		}
	}

	ctx = ContextWithAPIKeyPrincipal(ctx, row.ID.String(), row.Name.ValueOrEmpty(), row.WorkspaceID.String(), roles)

	// fire-and-forget: never block or fail auth on the last-used touch
	go func() { _ = db.Queries.TouchAPIKeyLastUsed(context.WithoutCancel(ctx), row.ID) }()

	return ctx, nil
}

// ContextWithAPIKeyPrincipal sets identity and the key's single workspace (with
// its roles) as both the membership set and the member workspace — API-key
// routes skip Membership(). Keys are never workspace owners. Permissions are
// resolved on demand by the authz package.
func ContextWithAPIKeyPrincipal(ctx context.Context, principalID, name, workspaceID string, roles []auth.RoleRef) context.Context {
	ctx = context.WithValue(ctx, userIDKey, principalID)
	ctx = context.WithValue(ctx, principalNameKey, name)
	ctx = context.WithValue(ctx, workspacesKey, []auth.WorkspaceClaim{{ID: workspaceID, Roles: roles}})
	ctx = context.WithValue(ctx, ctxWorkspaceID, workspaceID)
	ctx = context.WithValue(ctx, apiKeyPrincipalKey, true)
	return ctx
}

// Returns an HTTP middleware that validates access tokens,
// and attempts a refresh if the access token has expired.
func Authenticated() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := auth.ExtractBearerToken(r.Header.Get("Authorization"))
			if tokenStr == "" {
				http.Error(w, "Missing access token", http.StatusUnauthorized)
				return
			}

			if auth.IsAPIKey(tokenStr) {
				ctx, keyErr := buildAPIKeyContext(r.Context(), tokenStr)
				if keyErr != nil {
					http.Error(w, "Invalid API key", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			token, claims, err := auth.ValidateJWT(tokenStr)
			switch {
			case err == nil && token.Valid:
				ctx, ctxErr := buildAuthContext(r.Context(), claims.UserID, claims.Name, claims.Workspaces)
				if ctxErr != nil {
					http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
					return
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return

			case errors.Is(err, jwt.ErrTokenExpired):
				if claims == nil || claims.UserID == "" {
					http.Error(w, "Invalid access token", http.StatusUnauthorized)
					return
				}
				newTokens, userID, refreshErr := handleTokenRefresh(r, claims.UserID)
				if refreshErr != nil {
					http.Error(w, "Failed to refresh token", http.StatusUnauthorized)
					return
				}

				w.Header().Set("X-New-Access-Token", newTokens.AccessToken)
				w.Header().Set("X-New-Refresh-Token", newTokens.RefreshToken)

				_, newClaims, parseErr := auth.ValidateJWT(newTokens.AccessToken)
				var (
					ctx    context.Context
					ctxErr error
				)
				if parseErr == nil && newClaims != nil {
					ctx, ctxErr = buildAuthContext(r.Context(), newClaims.UserID, newClaims.Name, newClaims.Workspaces)
				} else {
					ctx, ctxErr = buildAuthContext(r.Context(), userID, "", nil)
				}
				if ctxErr != nil {
					http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
					return
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return

			default:
				http.Error(w, "Invalid access token", http.StatusUnauthorized)
				return
			}
		})
	}
}

func handleTokenRefresh(r *http.Request, userID string) (*TokenResponse, string, error) {
	refreshToken := r.Header.Get("X-Refresh-Token")
	deviceID := r.Header.Get("X-Device-ID")

	if refreshToken == "" || deviceID == "" {
		return nil, "", errors.New("missing refresh token or device ID")
	}

	newTokens, err := TryRefreshToken(r, refreshToken, deviceID, userID)
	if err != nil {
		return nil, "", err
	}
	return newTokens, userID, nil
}

func TryRefreshToken(r *http.Request, refreshToken string, deviceID string, userID string) (*TokenResponse, error) {
	ctx := r.Context()
	hashedToken := auth.HashRefreshToken(refreshToken, deviceID)

	// Validate refresh token in DB
	uid, err := db_types.NewJSONNullUUIDFromString(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse userID: %w", err)
	}
	tokenData, err := db.Queries.GetRefreshToken(ctx, generated.GetRefreshTokenParams{
		HashedToken: db_types.NewJSONNullString(hashedToken),
		UserID:      uid,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to look up refresh token: %w", err)
	}

	expiresAt := tokenData.ExpiresAt.ValueOrZero()
	if expiresAt.IsZero() {
		return nil, errors.New("refresh token expiry time is null")
	}

	if expiresAt.Before(time.Now()) {
		return nil, errors.New("refresh token has expired")
	}

	// Validate IP similarity
	currentIP := auth.GetIPAddress(r)
	tokenIP := tokenData.IssuedIp.IPNet.IP
	if !auth.SameSubnet(tokenIP, currentIP) {
		go auth.SendSecurityAlert(tokenData.UserID, tokenIP, currentIP)
	}

	// Revoke the old refresh token
	if err := db.Queries.DeleteRefreshToken(ctx, db_types.NewJSONNullString(hashedToken)); err != nil {
		return nil, fmt.Errorf("failed to revoke old refresh token: %w", err)
	}

	// Create new tokens
	newRefreshToken, err := auth.CreateRefreshToken(ctx, tokenData.UserID, deviceID, currentIP)
	if err != nil {
		return nil, fmt.Errorf("failed to create new refresh token: %w", err)
	}

	accessToken, err := auth.CreateJWT(ctx, tokenData.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to create access token: %w", err)
	}
	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: *newRefreshToken,
	}, nil
}
