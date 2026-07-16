package middlewares_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/db/db_types"
	"backend/e2e"
	"backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/middlewares"
)

func TestMain(m *testing.M) { e2e.Run(m) }

// A successful token refresh is a personal auth event: it persists as
// auth.token_refreshed, attributed to the user, with no workspace_id (an org
// admin never sees a member's login activity).
func TestTokenRefreshEmitsUserScopedEvent(t *testing.T) {
	f := e2e.Setup(t)
	ctx := context.Background()

	const (
		deviceID = "device-1"
		ip       = "192.0.2.10"
	)
	uid, err := db_types.NewJSONNullUUIDFromString(f.Actor.UserID)
	require.NoError(t, err)

	plain, err := auth.CreateRefreshToken(ctx, uid, deviceID, ip)
	require.NoError(t, err)

	r := httptestRequest(ip, deviceID)
	tokens, err := middlewares.TryRefreshToken(r, *plain, deviceID, f.Actor.UserID)
	require.NoError(t, err)
	require.NotEmpty(t, tokens.AccessToken)

	e2e.RequireEventStatus(t, f.Conn, audit.DomainAuth, audit.ActionTokenRefreshed, audit.StatusSuccess)

	var (
		wsNull      bool
		principalID string
	)
	require.NoError(t, f.Conn.QueryRow(
		`SELECT workspace_id IS NULL, principal_id
		   FROM audit.event
		  WHERE domain = $1 AND action = $2
		  ORDER BY occurred_at DESC LIMIT 1`,
		audit.DomainAuth, audit.ActionTokenRefreshed,
	).Scan(&wsNull, &principalID))

	require.True(t, wsNull, "auth events carry no workspace")
	require.Equal(t, f.Actor.UserID, principalID, "event attributed to the refreshing user")
}

func httptestRequest(ip, deviceID string) *http.Request {
	r, _ := http.NewRequest(http.MethodPost, "/refresh", nil)
	r.RemoteAddr = ip + ":54321"
	r.Header.Set("X-Device-ID", deviceID)
	return r
}
