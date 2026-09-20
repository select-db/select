package middlewares_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
)

// The branch every client takes every five minutes: the access token has
// expired, the refresh headers are present, and the request has to be served
// rather than bounced to the login screen.

// A request carrying an expired token is refreshed and served in one round
// trip, and the rotated tokens come back on the response for the client to
// keep. Standing is derived for it like any other request, so a role granted
// after the expired token was minted is enforced on this very request.
func TestExpiredToken_RefreshesAndCarriesFreshStanding(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	device := e2e.SignIn(t, memberID, "laptop")
	expired := e2e.MintExpiredJWT(t, memberID)

	roleID := e2e.SeedRoleWithPermission(t, f.Conn, f.Actor.WorkspaceID, "Key Manager", "workspace/api-keys.manage")
	e2e.SeedUserRole(t, f.Conn, memberID, roleID, f.Actor.WorkspaceID)

	rec := e2e.DoExpired(t, f.H, http.MethodPost, "/apikeys", expired, device, map[string]any{
		"workspace_id": f.Actor.WorkspaceID,
		"name":         "after-refresh",
		"role_ids":     []string{roleID},
	})

	require.Equalf(t, http.StatusOK, rec.Code,
		"a request with an expired token was not refreshed and served: %s", rec.Body.String())
	require.NotEmpty(t, rec.Header().Get("X-New-Access-Token"),
		"the client was not given the access token it now has to use")
	require.NotEmpty(t, rec.Header().Get("X-New-Refresh-Token"),
		"the old refresh token is spent and no replacement was returned")
}

// Without the refresh headers there is nothing to refresh from, and an expired
// token alone must not authenticate.
func TestExpiredToken_WithoutRefreshHeaders_IsRejected(t *testing.T) {
	f := e2e.Setup(t)

	rec := e2e.Do(t, f.H, http.MethodGet, "/datasources", e2e.MintExpiredJWT(t, f.Actor.UserID), nil)

	require.Equalf(t, http.StatusUnauthorized, rec.Code,
		"an expired token was accepted with no refresh token to back it: %s", rec.Body.String())
}

// The refresh token is bound to its device, so the wrong device id must not
// rotate it, even with an otherwise valid pair.
func TestExpiredToken_WrongDevice_IsRejected(t *testing.T) {
	f := e2e.Setup(t)

	device := e2e.SignIn(t, f.Actor.UserID, "laptop")
	impostor := &e2e.Device{UserID: device.UserID, DeviceID: "somewhere-else", Token: device.Token}

	rec := e2e.DoExpired(t, f.H, http.MethodGet, "/datasources",
		e2e.MintExpiredJWT(t, f.Actor.UserID), impostor, nil)

	require.Equalf(t, http.StatusUnauthorized, rec.Code,
		"a refresh token was accepted from a device it was not issued to: %s", rec.Body.String())
}
