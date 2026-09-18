package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"selectDb/internal/graph"
)

// Opening a folder writes select.config.json and then seeds the sample into it
// when there is nothing else there. The config is a row in the tree now, and
// counting it as content would leave every new workspace empty.
func TestOnlySelectsOwn(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []string
		want  bool
	}{
		{"nothing at all", nil, true},
		{"the config the open just wrote", []string{graph.WorkspaceConfigFileName}, true},
		{"a sidecar and a system artifact", []string{"a.sql.metadata.json", ".DS_Store"}, true},
		{"a query the user has", []string{graph.WorkspaceConfigFileName, "weekly.sql"}, false},
		{"a folder the user has", []string{"reports/"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range tc.files {
				if filepath.Base(name) != name {
					if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
						t.Fatalf("mkdir %s: %v", name, err)
					}
					continue
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read dir: %v", err)
			}
			if got := onlySelectsOwn(entries); got != tc.want {
				t.Errorf("onlySelectsOwn(%v) = %v, want %v", tc.files, got, tc.want)
			}
		})
	}
}
