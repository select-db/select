package e2e

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/auth"
	"backend/internal/middlewares"
)

// Device is a signed-in client: the refresh token one install holds, and the
// device id it was hashed with.
type Device struct {
	UserID   string
	DeviceID string
	Token    string
}

// SignIn issues a refresh token for one device, the way the login flow does.
func SignIn(t *testing.T, userID, deviceID string) *Device {
	t.Helper()
	token, err := auth.CreateRefreshToken(context.Background(), uuid.MustParse(userID), deviceID, "10.0.0.1")
	require.NoError(t, err)
	return &Device{UserID: userID, DeviceID: deviceID, Token: *token}
}

// Refresh runs the real refresh the middleware runs when an access token has
// expired, and rotates the device onto the token it hands back. The error is
// returned rather than asserted: whether a refresh succeeds is what the session
// tests are about.
func (d *Device) Refresh(t *testing.T) (*middlewares.TokenResponse, error) {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	tokens, err := middlewares.TryRefreshToken(req, d.Token, d.DeviceID, d.UserID)
	if err == nil {
		d.Token = tokens.RefreshToken
	}
	return tokens, err
}
