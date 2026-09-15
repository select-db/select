package graph

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()

	if _, err := ReadWorkspaceConfig(dir); !errors.Is(err, ErrNoWorkspaceConfig) {
		t.Fatalf("ReadWorkspaceConfig on an empty folder = %v, want ErrNoWorkspaceConfig", err)
	}

	if err := WriteWorkspaceConfig(dir, "app.select.dev", "ws-123"); err != nil {
		t.Fatalf("WriteWorkspaceConfig: %v", err)
	}
	cfg, err := ReadWorkspaceConfig(dir)
	if err != nil {
		t.Fatalf("ReadWorkspaceConfig: %v", err)
	}
	if cfg.Server != "app.select.dev" || cfg.WorkspaceID != "ws-123" {
		t.Errorf("round trip lost data: %+v", cfg)
	}
	if cfg.Version != workspaceConfigVersion {
		t.Errorf("version = %d, want %d", cfg.Version, workspaceConfigVersion)
	}

	// The file is committed and people read it. It should look like something a
	// person wrote, and say not to edit it.
	raw, err := os.ReadFile(WorkspaceConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Do not edit") {
		t.Error("config should carry its do-not-edit notice")
	}
	if !strings.HasSuffix(string(raw), "}\n") {
		t.Error("config should end with a newline")
	}

	if err := RemoveWorkspaceConfig(dir); err != nil {
		t.Fatalf("RemoveWorkspaceConfig: %v", err)
	}
	if _, err := ReadWorkspaceConfig(dir); !errors.Is(err, ErrNoWorkspaceConfig) {
		t.Errorf("folder should stop being a workspace once the config is gone, got %v", err)
	}
	// Removing what is not there is how "delete workspace" behaves after a
	// folder has already been cleaned up by hand, so it must not error.
	if err := RemoveWorkspaceConfig(dir); err != nil {
		t.Errorf("second RemoveWorkspaceConfig: %v", err)
	}
}

// A folder whose config is unusable is not a folder without a config: offering
// to create a fresh workspace over the first would lose whichever workspace the
// file named.
func TestReadWorkspaceConfigRejectsUnusableFiles(t *testing.T) {
	cases := map[string]string{
		"malformed":     "{not json",
		"no workspace":  `{"version":1,"server":"app.select.dev"}`,
		"no server":     `{"version":1,"workspaceId":"ws-1"}`,
		"newer format":  `{"version":99,"server":"s","workspaceId":"ws-1"}`,
		"empty strings": `{"version":1,"server":"","workspaceId":""}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, WorkspaceConfigFileName), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := ReadWorkspaceConfig(dir)
			if err == nil {
				t.Fatal("want an error")
			}
			if errors.Is(err, ErrNoWorkspaceConfig) {
				t.Fatal("an unusable config must not read as a missing one")
			}
		})
	}
}

// The config configures the folder; it is not a file to edit from the tree,
// same as db.config.json.
func TestWorkspaceConfigIsInternal(t *testing.T) {
	if !IsInternalWorkspaceFile(WorkspaceConfigFileName) {
		t.Errorf("%s should be hidden from the workspace tree", WorkspaceConfigFileName)
	}
}
