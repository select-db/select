package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/db"
	"backend/e2e"
	"backend/internal/auth"
)

// countRefreshTokens is how many refresh tokens the user currently holds,
// across every device.
func countRefreshTokens(t *testing.T, userID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.GetDB().QueryRow(
		`SELECT count(*) FROM auth.refresh_token WHERE user_id = $1::uuid`, userID,
	).Scan(&n))
	return n
}

func tokenExists(t *testing.T, plain *string, deviceID, userID string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.GetDB().QueryRow(
		`SELECT count(*) FROM auth.refresh_token WHERE hashed_token = $1 AND user_id = $2::uuid`,
		auth.HashRefreshToken(*plain, deviceID), userID,
	).Scan(&n))
	return n == 1
}

// A refresh token is hashed with the device that asked for it, so a user signed
// in on two machines holds one row each. Issuing a token for one device used to
// clear every other row for that user, which signed the other machine out on
// the next rotation, five minutes later.
func TestCreateRefreshToken_LeavesOtherDevicesSignedIn(t *testing.T) {
	conn := e2e.NewDB(t)
	actor := e2e.NewAccount(t, conn)
	userUUID := uuid.MustParse(actor.UserID)
	ctx := context.Background()

	laptop, err := auth.CreateRefreshToken(ctx, userUUID, "laptop", "10.0.0.1")
	require.NoError(t, err)
	desktop, err := auth.CreateRefreshToken(ctx, userUUID, "desktop", "10.0.0.2")
	require.NoError(t, err)

	require.True(t, tokenExists(t, laptop, "laptop", actor.UserID),
		"signing in on a second device must not sign the first one out")
	require.True(t, tokenExists(t, desktop, "desktop", actor.UserID))

	// The laptop rotates, as it does on every access-token expiry.
	rotated, err := auth.CreateRefreshToken(ctx, userUUID, "laptop", "10.0.0.1")
	require.NoError(t, err)

	require.True(t, tokenExists(t, rotated, "laptop", actor.UserID))
	require.True(t, tokenExists(t, desktop, "desktop", actor.UserID),
		"the laptop rotating must not sign the desktop out")
}

// The reap still runs, so rows do not accumulate forever: an expired token is
// cleared the next time the user is issued one.
func TestCreateRefreshToken_ReapsExpiredTokens(t *testing.T) {
	conn := e2e.NewDB(t)
	actor := e2e.NewAccount(t, conn)
	userUUID := uuid.MustParse(actor.UserID)

	_, err := conn.Exec(
		`INSERT INTO auth.refresh_token (hashed_token, user_id, expires_at)
		 VALUES ($1, $2::uuid, now() - interval '1 day')`,
		uuid.NewString(), actor.UserID)
	require.NoError(t, err)
	require.Equal(t, 1, countRefreshTokens(t, actor.UserID))

	_, err = auth.CreateRefreshToken(context.Background(), userUUID, "laptop", "10.0.0.1")
	require.NoError(t, err)

	require.Equal(t, 1, countRefreshTokens(t, actor.UserID),
		"the expired row is reaped, leaving only the new one")
}
