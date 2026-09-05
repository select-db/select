package fs_uri

import (
	"os"
	"path/filepath"
	"testing"
)

// Moved here with EnsureWithin itself, from internal/fs_provider.
func TestEnsureWithin(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "ws", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A symlink inside the workspace pointing out of it.
	link := filepath.Join(root, "ws", "evil")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	// Legit paths (existing dir, and a not-yet-created file under it) pass.
	for _, p := range []string{
		filepath.Join(root, "ws", "sub"),
		filepath.Join(root, "ws", "sub", "new.sql"),
		filepath.Join(root, "ws", "newdir", "f.txt"),
	} {
		if err := EnsureWithin(root, p); err != nil {
			t.Errorf("EnsureWithin(%q) = %v, want nil", p, err)
		}
	}

	// Anything via the escaping symlink is rejected.
	for _, p := range []string{
		link,
		filepath.Join(link, "passwd"),
	} {
		if err := EnsureWithin(root, p); err == nil {
			t.Errorf("EnsureWithin(%q) = nil, want escape error", p)
		}
	}
}
