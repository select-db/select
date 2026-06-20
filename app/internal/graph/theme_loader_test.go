package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetThemeFilePath_PointsAtUserDir(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	g := New(nil)
	path, err := g.GetThemeFilePath()
	if err != nil {
		t.Fatalf("GetThemeFilePath: %v", err)
	}
	dir, err := UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	want := filepath.Join(dir, ThemeFileName)
	if path != want {
		t.Errorf("GetThemeFilePath = %q, want per-user path %q", path, want)
	}
}

func TestLoadWorkspaceTheme_UserOverride(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	userDir, err := UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	content := ":root {\n  --fs-md: 99px;\n}\n"
	if err := os.WriteFile(filepath.Join(userDir, ThemeFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write user theme: %v", err)
	}

	g := New(nil)
	vars, err := g.LoadWorkspaceTheme()
	if err != nil {
		t.Fatalf("LoadWorkspaceTheme: %v", err)
	}
	if got := vars.Shared["--fs-md"]; got != "99px" {
		t.Errorf("--fs-md = %q, want 99px (user override)", got)
	}
	// A default value the user did not override should still be present.
	if len(vars.Light) == 0 && len(vars.Dark) == 0 {
		t.Error("expected default light/dark variables to remain after merge")
	}
}

// TestLoadWorkspaceTheme_IgnoresWorkspaceFile verifies appearance is user-owned:
// a .theme placed in the workspace root must not affect the loaded theme.
func TestLoadWorkspaceTheme_IgnoresWorkspaceFile(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	const workspaceID = "ws-theme"
	g := New(nil)
	g.WorkspaceGraph = &WorkspaceNode{ID: workspaceID, Type: "workspace"}

	wfs, err := NewWorkspaceFS(workspaceID)
	if err != nil {
		t.Fatalf("NewWorkspaceFS: %v", err)
	}
	if err := os.MkdirAll(wfs.WorkspaceRoot, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfs.WorkspaceRoot, ThemeFileName), []byte(":root {\n  --fs-md: 1px;\n}\n"), 0o644); err != nil {
		t.Fatalf("write workspace theme: %v", err)
	}

	vars, err := g.LoadWorkspaceTheme()
	if err != nil {
		t.Fatalf("LoadWorkspaceTheme: %v", err)
	}
	if vars.Shared["--fs-md"] == "1px" {
		t.Error("workspace .theme must be ignored; appearance is user-owned")
	}
}
