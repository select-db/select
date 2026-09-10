package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func writeIgnore(t testing.TB, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIgnoresDir(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root, `
# a comment, and a blank line above

node_modules
/build
dist/
target
docs/**/generated
vendor
!vendor/keep
`)

	m := newIgnoreMatcher(root)

	ignored := []string{
		"node_modules",              // bare name at the top
		"node_modules/svelte",       // and everything under it
		"packages/api/node_modules", // and at any depth
		"build",                     // anchored to the root
		"dist",                      // trailing slash, directories only
		"target",
		"docs/a/b/generated", // ** across segments
		"docs/generated",     // ** matching zero segments
		"vendor",
	}
	for _, rel := range ignored {
		if !m.IgnoresDir(filepath.Join(root, filepath.FromSlash(rel)), rel) {
			t.Errorf("%s should be ignored", rel)
		}
	}

	kept := []string{
		"queries",
		"packages/api/src",
		"sub/build",   // "/build" is anchored, so this one is not it
		"vendor/keep", // brought back by the negation
		"docs/generated-notes",
	}
	for _, rel := range kept {
		if m.IgnoresDir(filepath.Join(root, filepath.FromSlash(rel)), rel) {
			t.Errorf("%s should not be ignored", rel)
		}
	}

	if m.IgnoresDir(root, ".") {
		t.Error("the workspace root should never be ignored")
	}
}

func TestIgnoresDir_NestedFileWins(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root, "generated\n")
	writeIgnore(t, filepath.Join(root, "packages", "api"), "!generated\n")

	m := newIgnoreMatcher(root)

	if !m.IgnoresDir(filepath.Join(root, "web", "generated"), "web/generated") {
		t.Error("the root rule should still apply where nothing overrides it")
	}
	if m.IgnoresDir(filepath.Join(root, "packages", "api", "generated"), "packages/api/generated") {
		t.Error("the nested .gitignore should win")
	}
}

// An edit takes effect on the next matcher, not mid-walk: half a tree filtered
// by the old rules and half by the new is worse than either.
func TestIgnoresDir_IsASnapshotPerMatcher(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root, "build\n")

	m := newIgnoreMatcher(root)
	if !m.IgnoresDir(filepath.Join(root, "build"), "build") {
		t.Fatal("build should start out ignored")
	}

	writeIgnore(t, root, "dist\n")

	if !m.IgnoresDir(filepath.Join(root, "build"), "build") {
		t.Error("the same matcher should keep the answer it already gave")
	}

	fresh := newIgnoreMatcher(root)
	if fresh.IgnoresDir(filepath.Join(root, "build"), "build") {
		t.Error("a new matcher should stop ignoring build")
	}
	if !fresh.IgnoresDir(filepath.Join(root, "dist"), "dist") {
		t.Error("a new matcher should start ignoring dist")
	}
}

func TestIgnoresDir_NoFile(t *testing.T) {
	root := t.TempDir()
	m := newIgnoreMatcher(root)

	if m.IgnoresDir(filepath.Join(root, "node_modules"), "node_modules") {
		t.Error("without a .gitignore nothing is ignored, not even node_modules")
	}
}

func TestWalkSkipsIgnoredDirectoriesButKeepsFiles(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root, "node_modules\n.env\n")

	for _, rel := range []string{
		"queries/top.sql",
		".env",
		"node_modules/pkg/index.js",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	fsCtx := NewWorkspaceFSFromRoot("ws-1", root)

	seen := map[string]bool{}
	if err := fsCtx.Walk(func(e Entry) error {
		seen[e.Rel] = true
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if !seen["queries/top.sql"] {
		t.Error("an ordinary file should be walked")
	}
	// Gitignored, but the app reads it for $VARIABLES and offers it in the tree.
	if !seen[".env"] {
		t.Error(".env is gitignored by default and must still be walked")
	}
	if seen["node_modules"] || seen["node_modules/pkg"] || seen["node_modules/pkg/index.js"] {
		t.Error("an ignored directory and its contents should be skipped entirely")
	}
}
