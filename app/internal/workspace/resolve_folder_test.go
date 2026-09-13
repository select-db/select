package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFolder_AcceptsAnOrdinaryFolder(t *testing.T) {
	dir := t.TempDir()

	got, err := resolveFolder(dir)
	if err != nil {
		t.Fatalf("resolveFolder(%q): %v", dir, err)
	}
	if got != filepath.Clean(dir) {
		t.Errorf("got %q, want %q", got, filepath.Clean(dir))
	}

	// Must come back absolute: everything downstream joins onto it.
	t.Chdir(dir)
	got, err = resolveFolder(".")
	if err != nil {
		t.Fatalf("resolveFolder(\".\"): %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("got %q, want an absolute path", got)
	}
}

func TestResolveFolder_RefusesWhatIsNotAFolderToOpen(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(file, []byte("select 1"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := dir
	for filepath.Dir(root) != root {
		root = filepath.Dir(root)
	}

	cases := map[string]string{
		"nothing given":   "",
		"does not exist":  filepath.Join(dir, "nope"),
		"a file":          file,
		"filesystem root": root,
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		cases["home folder"] = home
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := resolveFolder(path); err == nil {
				t.Errorf("resolveFolder(%q) = nil error, want rejection", path)
			}
		})
	}
}
