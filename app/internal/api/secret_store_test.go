package api

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestReadCredentialStatus covers the other half of the "signed out every few
// minutes" report: the session poll runs twice a second, and it used to read any
// keyring error -- a locked keychain, no D-Bus on the session bus, a transient
// refusal -- as the credentials being gone.
func TestReadCredentialStatus(t *testing.T) {
	// GetAppDataDir, which the token keys hang off, writes under the config dir.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	t.Run("an unreachable keyring says nothing either way", func(t *testing.T) {
		keyring.MockInitWithError(errors.New("secret service unavailable"))
		t.Cleanup(keyring.MockInit)

		if got := ReadCredentialStatus(); got != CredentialsUnreadable {
			t.Fatalf("a keyring that could not answer must not read as signed out, got %v", got)
		}
	})

	t.Run("credentials the keyring reports as gone are a logout", func(t *testing.T) {
		keyring.MockInit()

		if got := ReadCredentialStatus(); got != CredentialsMissing {
			t.Fatalf("an empty keyring is a signed-out session, got %v", got)
		}
	})

	t.Run("a complete pair is a live session", func(t *testing.T) {
		keyring.MockInit()
		if err := SaveAccessToken("accessX"); err != nil {
			t.Fatalf("save access token: %v", err)
		}
		if err := SaveRefreshToken("refreshX"); err != nil {
			t.Fatalf("save refresh token: %v", err)
		}

		if got := ReadCredentialStatus(); got != CredentialsPresent {
			t.Fatalf("a stored pair is a live session, got %v", got)
		}
	})
}

// TestClearRefreshTokenForgetsInProcessCopy pins the guarantee doWithRetry and
// Logout both rely on: a signed-out token is never presented on the session that
// follows, even when it only ever lived in memory because a keyring write failed.
func TestClearRefreshTokenForgetsInProcessCopy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	keyring.MockInit()

	memTokenMu.Lock()
	memRefreshToken = "rotated-but-never-persisted"
	memTokenMu.Unlock()
	t.Cleanup(forgetRefreshToken)

	_ = ClearRefreshToken()

	if got, _ := currentRefreshToken(); got != "" {
		t.Fatalf("the in-process copy should be gone with the session, got %q", got)
	}
}
