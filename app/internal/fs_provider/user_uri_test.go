package fs_provider

import (
	"os"
	"path/filepath"
	"testing"

	"selectDb/internal/utils"
)

// withTempUserConfig points GetAppDataDir at a temp dir so user URIs resolve
// into an isolated location.
func withTempUserConfig(t *testing.T) string {
	t.Helper()
	oldHome := os.Getenv("HOME")
	oldEnv := os.Getenv("APP_ENV")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")

	t.Setenv("HOME", t.TempDir())
	t.Setenv("APP_ENV", "test-user-uri")
	_ = os.Unsetenv("XDG_CONFIG_HOME")

	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("APP_ENV", oldEnv)
		if oldXDG != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	})

	dir, err := utils.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	return dir
}

func TestGetOSPathFromURI_UserScheme(t *testing.T) {
	userDir := withTempUserConfig(t)
	fsp := New() // no root set: user URIs must not depend on the server root

	got, err := fsp.GetOSPathFromURI("selectdb://user/.theme")
	if err != nil {
		t.Fatalf("GetOSPathFromURI(user/.theme): %v", err)
	}
	want := filepath.Join(userDir, ".theme")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetOSPathFromURI_UserSchemeRejectsTraversal(t *testing.T) {
	withTempUserConfig(t)
	fsp := New()

	for _, uri := range []string{
		"selectdb://user/../escape",
		"selectdb://user/",
	} {
		if _, err := fsp.GetOSPathFromURI(uri); err == nil {
			t.Errorf("GetOSPathFromURI(%q) = nil error, want rejection", uri)
		}
	}
}

func TestUserURI_ReadWriteRoundTrip(t *testing.T) {
	withTempUserConfig(t)
	fsp := New()

	const uri = "selectdb://user/.config"
	if err := fsp.Write(WriteParams{URI: uri, Content: "hello"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := fsp.ReadFile(ReadFileParams{URI: uri})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != "hello" {
		t.Errorf("ReadFile = %q, want %q", got, "hello")
	}
}
