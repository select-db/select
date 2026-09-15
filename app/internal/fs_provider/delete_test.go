package fs_provider

import (
	"os"
	"path/filepath"
	"testing"
)

// deleteFixture returns a provider rooted at a temp dir, and the URI naming a
// file inside it that already has its result-metadata sidecar beside it.
func deleteFixture(t *testing.T) (*FSProvider, string, string, string) {
	t.Helper()

	dir := t.TempDir()

	path := filepath.Join(dir, "query.sql")
	meta := path + MetadataFileSuffix
	for _, p := range []string{path, meta} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	fsp := New(func(id string) (string, error) {
		if id != "ws-1" {
			return "", os.ErrNotExist
		}
		return dir, nil
	})
	return fsp, "selectdb://workspaces/ws-1/query.sql", path, meta
}

func TestDelete_TakesTheMetadataSidecarWithTheFile(t *testing.T) {
	fsp, uri, path, meta := deleteFixture(t)

	if err := fsp.Delete(DeleteParams{URI: uri}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// The sidecar has no entry of its own in the tree, so leaving it behind
	// leaves a file nothing can ever reach to remove.
	for _, p := range []string{path, meta} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists after delete", filepath.Base(p))
		}
	}
}

func TestDelete_FileWithNoSidecarIsNotAnError(t *testing.T) {
	fsp, uri, path, meta := deleteFixture(t)
	if err := os.Remove(meta); err != nil {
		t.Fatalf("remove sidecar: %v", err)
	}

	if err := fsp.Delete(DeleteParams{URI: uri}); err != nil {
		t.Fatalf("delete without a sidecar should succeed, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still exists after delete")
	}
}

func TestDelete_MissingFileStaysANoOp(t *testing.T) {
	fsp, _, _, _ := deleteFixture(t)

	if err := fsp.Delete(DeleteParams{URI: "selectdb://workspaces/ws-1/gone.sql"}); err != nil {
		t.Fatalf("deleting a missing file should succeed, got %v", err)
	}
}
