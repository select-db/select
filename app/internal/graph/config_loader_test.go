package graph

import (
	"os"
	"path/filepath"
	"testing"

	"selectDb/internal/keymap"
)

func findKeybinding(bindings []keymap.Binding, command string) (keymap.Binding, bool) {
	for _, kb := range bindings {
		if kb.Command == command {
			return kb, true
		}
	}
	return keymap.Binding{}, false
}

func TestLoadConfig_Defaults(t *testing.T) {
	withTempAppDataDir(t)

	g := New(nil)
	cfg, err := g.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Keybindings) == 0 {
		t.Error("expected default keybindings to be present")
	}
	if len(cfg.EditorSnippets) == 0 {
		t.Error("expected default editor snippets to be present")
	}
}

func TestLoadConfig_UserOverridesKeybindingsAndSnippets(t *testing.T) {
	withTempAppDataDir(t)

	userDir, err := UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	content := `{
      "keybindings": { "workbench": [ { "key": "cmd+q", "command": "workbench.userBinding" } ] },
      "editor_snippets": [ { "prefix": "uu", "body": "USER", "description": "user snippet" } ]
    }`
	if err := os.WriteFile(filepath.Join(userDir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	g := New(nil)
	cfg, err := g.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
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
	withTempAppDataDir(t)

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
	withTempAppDataDir(t)

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

func TestNormalizeExecutionLimits(t *testing.T) {
	cases := []struct {
		name              string
		inTimeout, inSize int
		wantTimeout       int
		wantSize          int
	}{
		{"zero -> defaults", 0, 0, DefaultStatementTimeoutMs, DefaultMaxResultSizeMB},
		{"negative -> defaults", -5, -5, DefaultStatementTimeoutMs, DefaultMaxResultSizeMB},
		{"valid kept", 5000, 42, 5000, 42},
		{"size capped", 5000, 9999, 5000, MaxMaxResultSizeMB},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeStatementTimeoutMs(c.inTimeout); got != c.wantTimeout {
				t.Errorf("NormalizeStatementTimeoutMs(%d) = %d, want %d", c.inTimeout, got, c.wantTimeout)
			}
			if got := NormalizeMaxResultSizeMB(c.inSize); got != c.wantSize {
				t.Errorf("NormalizeMaxResultSizeMB(%d) = %d, want %d", c.inSize, got, c.wantSize)
			}
		})
	}
}

func TestWorkspaceExecutionLimits_FromNode(t *testing.T) {
	g := New(nil)
	g.WorkspaceGraph = &WorkspaceNode{ID: "ws", Type: "workspace", StatementTimeoutMs: 7000, MaxResultSizeMB: 80}

	timeout, size := g.WorkspaceExecutionLimits()
	if timeout != 7000 || size != 80 {
		t.Errorf("WorkspaceExecutionLimits = (%d, %d), want (7000, 80)", timeout, size)
	}
}

func TestWorkspaceExecutionLimits_UnsetNodeFallsBackToDefaults(t *testing.T) {
	g := New(nil)
	g.WorkspaceGraph = &WorkspaceNode{ID: "ws", Type: "workspace"} // limits zero

	timeout, size := g.WorkspaceExecutionLimits()
	if timeout != DefaultStatementTimeoutMs || size != DefaultMaxResultSizeMB {
		t.Errorf("WorkspaceExecutionLimits = (%d, %d), want defaults (%d, %d)", timeout, size, DefaultStatementTimeoutMs, DefaultMaxResultSizeMB)
	}
}

// The defaults ship inside the binary, so a binding in them that cannot be
// parsed is a keystroke nobody can press and nobody is told about. Resolved for
// every platform, because each one is somebody's.
func TestDefaultKeybindingsResolveOnEveryPlatform(t *testing.T) {
	defaults, err := parseConfig(DefaultUserConfigContent)
	if err != nil {
		t.Fatalf("parse default user config: %v", err)
	}

	for _, platform := range []keymap.Platform{keymap.MacOS, keymap.Linux, keymap.Windows} {
		bindings, problems := keymap.Resolve(defaults.Keybindings, nil, platform)
		for _, problem := range problems {
			t.Errorf("%s: %s %q: %s", platform, problem.Level, problem.Key, problem.Message)
		}
		if len(bindings) == 0 {
			t.Errorf("%s: no default keybindings resolved", platform)
		}
	}
}

// Two defaults on the same chord under the same condition is one of them
// quietly winning. It is how a shortcut ends up doing something else.
func TestDefaultKeybindingsDoNotCollide(t *testing.T) {
	defaults, err := parseConfig(DefaultUserConfigContent)
	if err != nil {
		t.Fatalf("parse default user config: %v", err)
	}

	for _, platform := range []keymap.Platform{keymap.MacOS, keymap.Linux, keymap.Windows} {
		bindings, _ := keymap.Resolve(defaults.Keybindings, nil, platform)

		seen := make(map[string]string, len(bindings))
		for _, binding := range bindings {
			chord := binding.Key + " when " + binding.When
			if previous, taken := seen[chord]; taken && previous != binding.Command {
				t.Errorf("%s: %s is bound to both %s and %s", platform, chord, previous, binding.Command)
			}
			seen[chord] = binding.Command
		}
	}
}
