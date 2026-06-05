package fs_provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureWithinRoot(t *testing.T) {
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
		if err := ensureWithinRoot(root, p); err != nil {
			t.Errorf("ensureWithinRoot(%q) = %v, want nil", p, err)
		}
	}

	// Anything via the escaping symlink is rejected.
	for _, p := range []string{
		link,
		filepath.Join(link, "passwd"),
	} {
		if err := ensureWithinRoot(root, p); err == nil {
			t.Errorf("ensureWithinRoot(%q) = nil, want escape error", p)
		}
	}
}
