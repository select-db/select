package e2e

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// MintExpiredJWT signs an access token that expired an hour ago, with the key
// the harness wrote. CreateJWT cannot produce one, and the middleware's refresh
// branch runs only for a token in this state.
func MintExpiredJWT(t *testing.T, userID string) string {
	t.Helper()
	pemBytes, err := os.ReadFile(os.Getenv("PRIVATE_KEY_PATH"))
	require.NoError(t, err)
	block, _ := pem.Decode(pemBytes)
	require.NotNil(t, block, "no PEM block in the harness signing key")
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)

	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, auth.CustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    auth.Issuer,
			Audience:  jwt.ClaimStrings{auth.Audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}).SignedString(key)
	require.NoError(t, err)
	return signed
}

// DoExpired sends a request the way a client does when its access token has
// just expired: the stale bearer plus the refresh headers the middleware reads.
func DoExpired(t *testing.T, h http.Handler, method, path, expiredToken string, d *Device, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	req.Header.Set("X-Refresh-Token", d.Token)
	req.Header.Set("X-Device-ID", d.DeviceID)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
