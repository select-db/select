package api

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestCredentialsCleared covers the other half of the "signed out every few
// minutes" report: the session poll runs twice a second, and it used to read any
// keyring error -- a locked keychain, no D-Bus on the session bus, a transient
// refusal -- as the credentials being gone.
func TestCredentialsCleared(t *testing.T) {
	// GetAppDataDir, which the token keys hang off, writes under the config dir.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	t.Run("an unreachable keyring is not a logout", func(t *testing.T) {
		keyring.MockInitWithError(errors.New("secret service unavailable"))
		t.Cleanup(keyring.MockInit)

		if CredentialsCleared() {
			t.Fatal("a keyring that could not answer must not read as a signed-out session")
		}
	})

	t.Run("credentials the keyring reports as gone are a logout", func(t *testing.T) {
		keyring.MockInit()

		if !CredentialsCleared() {
			t.Fatal("an empty keyring is a signed-out session")
		}
	})

	t.Run("a complete pair keeps the session", func(t *testing.T) {
		keyring.MockInit()
		if err := SaveAccessToken("accessX"); err != nil {
			t.Fatalf("save access token: %v", err)
		}
		if err := SaveRefreshToken("refreshX"); err != nil {
			t.Fatalf("save refresh token: %v", err)
		}

		if CredentialsCleared() {
			t.Fatal("a stored pair is a live session")
		}
	})
}
