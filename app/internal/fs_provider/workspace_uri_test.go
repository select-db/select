package fs_provider

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// twoWorkspaces returns a provider over two workspaces in unrelated folders.
func twoWorkspaces(t *testing.T) (*FSProvider, string, string) {
	t.Helper()

	a, b := t.TempDir(), t.TempDir()
	fsp := New(func(id string) (string, error) {
		switch id {
		case "ws-a":
			return a, nil
		case "ws-b":
			return b, nil
		}
		return "", fmt.Errorf("unknown workspace %s", id)
	})
	return fsp, a, b
}

func TestGetOSPathFromURI_ResolvesEachWorkspaceInItsOwnFolder(t *testing.T) {
	fsp, a, b := twoWorkspaces(t)

	for _, tc := range []struct {
		uri  string
		want string
	}{
		{"selectdb://workspaces/ws-a", a},
		{"selectdb://workspaces/ws-a/queries/top.sql", filepath.Join(a, "queries", "top.sql")},
		{"selectdb://workspaces/ws-b/queries/top.sql", filepath.Join(b, "queries", "top.sql")},
	} {
		got, err := fsp.GetOSPathFromURI(tc.uri)
		if err != nil {
			t.Fatalf("GetOSPathFromURI(%q): %v", tc.uri, err)
		}
		if got != tc.want {
			t.Errorf("GetOSPathFromURI(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}

func TestGetOSPathFromURI_RejectsWhatItCannotResolve(t *testing.T) {
	fsp, _, _ := twoWorkspaces(t)

	for _, uri := range []string{
		"selectdb://workspaces/ws-c/query.sql", // no such workspace
		"selectdb://workspaces/",               // names no workspace
		"selectdb://workspaces",                // ditto
		"selectdb://elsewhere/query.sql",       // neither namespace
		"not-a-uri",
	} {
		if _, err := fsp.GetOSPathFromURI(uri); err == nil {
			t.Errorf("GetOSPathFromURI(%q) = nil error, want rejection", uri)
		}
	}
}

// Containment is per workspace, not per server: climbing out reaches the user's
// own files.
func TestGetOSPathFromURI_CannotClimbOutOfAWorkspace(t *testing.T) {
	fsp, a, b := twoWorkspaces(t)

	rel, err := filepath.Rel(a, b)
	if err != nil {
		t.Skipf("temp dirs are not relative to one another: %v", err)
	}

	for _, uri := range []string{
		"selectdb://workspaces/ws-a/../escape",
		"selectdb://workspaces/ws-a/" + filepath.ToSlash(rel) + "/stolen.sql",
	} {
		if _, err := fsp.GetOSPathFromURI(uri); err == nil {
			t.Errorf("GetOSPathFromURI(%q) = nil error, want rejection", uri)
		}
	}
}

// A symlink from a cloned repo must not be a way out either.
func TestGetOSPathFromURI_DoesNotFollowSymlinksOut(t *testing.T) {
	fsp, a, b := twoWorkspaces(t)

	if err := os.Symlink(b, filepath.Join(a, "escape")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if _, err := fsp.GetOSPathFromURI("selectdb://workspaces/ws-a/escape/stolen.sql"); err == nil {
		t.Error("a symlink out of the workspace should not resolve")
	}
}
