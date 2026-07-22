package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/e2e"
	"backend/internal/audit"
	"backend/internal/auth"
)

func TestMain(m *testing.M) { e2e.Run(m) }

// mockGitHub stands up a fake GitHub returning a successful device-code exchange
// and a user with the given email, and points the auth package at it.
func mockGitHub(t *testing.T, email string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "gho_test", "token_type": "bearer"})
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login": "octocat", "id": 12345, "name": "Octo Cat", "email": email, "avatar_url": "http://avatar.local",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	auth.SetGitHubURLsForTest(srv.URL+"/login/oauth/access_token", srv.URL+"/user", srv.URL+"/user/emails")
}

func postAccessToken(t *testing.T, h http.Handler, code, deviceID string) int {
	t.Helper()
	rec := e2e.Do(t, h, http.MethodPost, "/auth/get-access-token", "",
		map[string]any{"device_code": code, "device_id": deviceID})
	return rec.Code
}

// A successful device-code login persists auth.login as the user's own event:
// attributed to the new user, with no workspace_id.
func TestLoginEmitsUserScopedEvent(t *testing.T) {
	f := e2e.Setup(t)
	t.Setenv("GITHUB_CLIENT_ID", "test-client")
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	mockGitHub(t, "octocat@allowed.example")

	const code = "device-code-success"
	auth.RememberDeviceCodeForTest(code)

	require.Equal(t, http.StatusOK, postAccessToken(t, f.H, code, "dev-1"))
	e2e.RequireEventStatus(t, f.Conn, audit.DomainAuth, audit.ActionLogin, audit.StatusSuccess)

	var (
		wsNull      bool
		principalID string
	)
	require.NoError(t, f.Conn.QueryRow(
		`SELECT workspace_id IS NULL, principal_id
		   FROM audit.event
		  WHERE domain = $1 AND action = $2
		  ORDER BY occurred_at DESC LIMIT 1`,
		audit.DomainAuth, audit.ActionLogin,
	).Scan(&wsNull, &principalID))
	require.True(t, wsNull, "auth events carry no workspace")
	require.NotEmpty(t, principalID, "login attributed to the signing-in user")
}

// A GitHub identity the instance allowlist refuses persists auth.login_failed
// (status failure) with the attempted email, and the request is forbidden.
func TestLoginFailedOnDisallowedEmail(t *testing.T) {
	f := e2e.Setup(t)
	t.Setenv("GITHUB_CLIENT_ID", "test-client")
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ALLOWED_USERS", "@allowed.example")
	mockGitHub(t, "intruder@blocked.example")

	const code = "device-code-denied"
	auth.RememberDeviceCodeForTest(code)

	require.Equal(t, http.StatusForbidden, postAccessToken(t, f.H, code, "dev-2"))
	e2e.RequireEventStatus(t, f.Conn, audit.DomainAuth, audit.ActionLoginFailed, audit.StatusFailure)

	var email string
	require.NoError(t, f.Conn.QueryRow(
		`SELECT payload->>'email'
		   FROM audit.event
		  WHERE domain = $1 AND action = $2
		  ORDER BY occurred_at DESC LIMIT 1`,
		audit.DomainAuth, audit.ActionLoginFailed,
	).Scan(&email))
	require.Equal(t, "intruder@blocked.example", email)
}
