package graph

import (
	"os"
	"path/filepath"
	"testing"
)

// setupConfigWorkspace configures a temp app data dir, a current server and a
// workspace graph + on-disk workspace root, returning the workspace root and
// the per-user config dir so tests can write workspace and user .config files.
func setupConfigWorkspace(t *testing.T) (g *Graph, workspaceRoot, userDir string) {
	t.Helper()

	_, restore := withTempAppDataDir(t)
	t.Cleanup(restore)

	const workspaceID = "ws-config"
	g = New(nil)
	g.WorkspaceGraph = &WorkspaceNode{ID: workspaceID, Type: "workspace"}

	wfs, err := NewWorkspaceFS(workspaceID)
	if err != nil {
		t.Fatalf("NewWorkspaceFS: %v", err)
	}
	workspaceRoot = wfs.WorkspaceRoot
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}

	userDir, err = UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	return g, workspaceRoot, userDir
}

func findKeybinding(cfg []Keybinding, command string) (Keybinding, bool) {
	for _, kb := range cfg {
		if kb.Command == command {
			return kb, true
		}
	}
	return Keybinding{}, false
}

func TestLoadWorkspaceConfig_Defaults(t *testing.T) {
	g, _, _ := setupConfigWorkspace(t)

	cfg, err := g.LoadWorkspaceConfig()
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}

	if cfg.StatementTimeoutMs != 30000 {
		t.Errorf("StatementTimeoutMs = %d, want 30000", cfg.StatementTimeoutMs)
	}
	if cfg.MaxResultSizeMB != 100 {
		t.Errorf("MaxResultSizeMB = %d, want 100", cfg.MaxResultSizeMB)
	}
	if len(cfg.Keybindings) == 0 {
		t.Error("expected default keybindings to be present")
	}
	if len(cfg.EditorSnippets) == 0 {
		t.Error("expected default editor snippets to be present")
	}
}

func TestLoadWorkspaceConfig_WorkspaceOwnsExecutionLimits(t *testing.T) {
	g, workspaceRoot, _ := setupConfigWorkspace(t)

	// Workspace .config tries to set both limits AND a keybinding. Only the
	// limits must be honoured; keybindings are user-owned and must be ignored.
	content := `{
      "statement_timeout_ms": 5000,
      "max_result_size_mb": 42,
      "keybindings": { "workbench": [ { "key": "cmd+x", "command": "workbench.shouldBeIgnored" } ] }
    }`
	if err := os.WriteFile(filepath.Join(workspaceRoot, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}

	cfg, err := g.LoadWorkspaceConfig()
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}

	if cfg.StatementTimeoutMs != 5000 {
		t.Errorf("StatementTimeoutMs = %d, want 5000 (workspace wins)", cfg.StatementTimeoutMs)
	}
	if cfg.MaxResultSizeMB != 42 {
		t.Errorf("MaxResultSizeMB = %d, want 42 (workspace wins)", cfg.MaxResultSizeMB)
	}
	if _, ok := findKeybinding(cfg.Keybindings, "workbench.shouldBeIgnored"); ok {
		t.Error("workspace .config must not contribute keybindings")
	}
}

func TestLoadWorkspaceConfig_UserOwnsKeybindingsAndSnippets(t *testing.T) {
	g, _, userDir := setupConfigWorkspace(t)

	// User .config tries to set keybindings/snippets AND execution limits. Only
	// keybindings/snippets must be honoured; limits are workspace-owned.
	content := `{
      "statement_timeout_ms": 999,
      "keybindings": { "workbench": [ { "key": "cmd+q", "command": "workbench.userBinding" } ] },
      "editor_snippets": [ { "prefix": "uu", "body": "USER", "description": "user snippet" } ]
    }`
	if err := os.WriteFile(filepath.Join(userDir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	cfg, err := g.LoadWorkspaceConfig()
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}

	if cfg.StatementTimeoutMs != 30000 {
		t.Errorf("StatementTimeoutMs = %d, want 30000 (user .config must not set limits)", cfg.StatementTimeoutMs)
	}
	if _, ok := findKeybinding(cfg.Keybindings, "workbench.userBinding"); !ok {
		t.Error("expected user keybinding to be present")
	}

	var foundSnippet bool
	for _, s := range cfg.EditorSnippets {
		if s.Prefix == "uu" && s.Body == "USER" {
			foundSnippet = true
		}
	}
	if !foundSnippet {
		t.Error("expected user editor snippet to be present")
	}
}

func TestGetUserConfigFilePath(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	path, err := GetUserConfigFilePath()
	if err != nil {
		t.Fatalf("GetUserConfigFilePath: %v", err)
	}
	dir, err := UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	want := filepath.Join(dir, ConfigFileName)
	if path != want {
		t.Errorf("GetUserConfigFilePath = %q, want %q", path, want)
	}
}

func TestResetUserConfig(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	if err := ResetUserConfig(); err != nil {
		t.Fatalf("ResetUserConfig: %v", err)
	}
	path, _ := GetUserConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read reset user config: %v", err)
	}
	if string(data) != DefaultUserConfigContent {
		t.Error("ResetUserConfig did not write the default user config content")
	}
}
