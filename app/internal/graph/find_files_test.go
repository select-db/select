package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"selectDb/internal/server"
)

// queryWorkspace lays out a workspace from a set of relative paths and returns
// a built graph over it.
func queryWorkspace(t *testing.T, workspaceID string, files ...string) (*Graph, *WorkspaceFS) {
	t.Helper()

	serverRoot, err := server.CurrentServerRoot()
	if err != nil {
		t.Fatalf("CurrentServerRoot: %v", err)
	}
	workspaceRoot := filepath.Join(serverRoot, "workspaces", workspaceID)

	for _, rel := range files {
		path := filepath.Join(workspaceRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	g := &Graph{WorkspaceGraph: &WorkspaceNode{ID: workspaceID, Type: "workspace"}}
	fsCtx := NewWorkspaceFSFromRoot(workspaceID, workspaceRoot)
	if err := g.buildWorkspaceGraphFromFS(fsCtx); err != nil {
		t.Fatalf("build: %v", err)
	}

	return g, fsCtx
}

func queryNames(t *testing.T, g *Graph, q FileQuery) []string {
	t.Helper()

	files, err := g.FindFiles(t.Context(), q)
	if err != nil {
		t.Fatalf("FindFiles(%+v): %v", q, err)
	}

	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

func TestFindFiles_SeesFoldersNobodyOpened(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	g, fsCtx := queryWorkspace(t, "ws-find",
		"root.sql", "unopened/child.sql", "unopened/deeper/deep.sql")

	names := queryNames(t, g, FileQuery{})
	if len(names) != 3 {
		t.Fatalf("expected every file, got %v", names)
	}

	// And the query leaves the graph exactly as it was.
	if folder, _ := g.lookup(fsCtx.URI("unopened")).(*FolderNode); folder == nil || folder.Resolved {
		t.Errorf("a query should not open a folder, got %+v", folder)
	}
}

func TestFindFiles_RanksByHowWellTheNameMatches(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	g, _ := queryWorkspace(t, "ws-rank",
		"users.sql",             // exact
		"users_by_country.sql",  // prefix
		"active_users.sql",      // word start
		"deactivateduserss.sql", // contains
		"users/unrelated.sql",   // path only
		"orders.sql")            // no match

	names := queryNames(t, g, FileQuery{Pattern: "users"})

	want := []string{"users.sql", "users_by_country.sql", "active_users.sql", "deactivateduserss.sql", "unrelated.sql"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Errorf("rank %d: got %q want %q (full: %v)", i, names[i], name, names)
		}
	}
}

func TestFindFiles_MatchesCaseInsensitively(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	g, _ := queryWorkspace(t, "ws-case", "Users.SQL")

	if names := queryNames(t, g, FileQuery{Pattern: "users"}); len(names) != 1 {
		t.Errorf("expected a case-insensitive match, got %v", names)
	}
}

func TestFindFiles_KeepsTheBestWithinTheLimit(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	paths := []string{"report.sql"} // the exact match, written first
	for i := 0; i < 50; i++ {
		paths = append(paths, fmt.Sprintf("reporting_%02d.sql", i))
	}
	g, _ := queryWorkspace(t, "ws-limit", paths...)

	names := queryNames(t, g, FileQuery{Pattern: "report", Limit: 3})
	if len(names) != 3 {
		t.Fatalf("expected the limit to be honoured, got %d: %v", len(names), names)
	}
	// The cap keeps the best matches, not the first ones the walk happened on.
	if names[0] != "report.sql" {
		t.Errorf("expected the exact match to survive the cap, got %v", names)
	}
}

func TestFindFiles_ScopesToAFolderAndADepth(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	g, fsCtx := queryWorkspace(t, "ws-scope",
		"queries/a.sql", "queries/nested/b.sql", "elsewhere/c.sql")

	subtree := queryNames(t, g, FileQuery{FolderURI: fsCtx.URI("queries")})
	if len(subtree) != 2 {
		t.Errorf("expected the folder's subtree, got %v", subtree)
	}

	// Depth 1 is what a $ref needs: the folder's own files, nothing deeper.
	own := queryNames(t, g, FileQuery{FolderURI: fsCtx.URI("queries"), Depth: 1})
	if len(own) != 1 || own[0] != "a.sql" {
		t.Errorf("expected only the folder's own files, got %v", own)
	}

	if outside := queryNames(t, g, FileQuery{FolderURI: "selectdb://workspaces/other/x"}); len(outside) != 0 {
		t.Errorf("a scope outside the workspace should match nothing, got %v", outside)
	}
}

func TestFindFiles_FiltersByExtensionAndSkipsInternalFiles(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	g, _ := queryWorkspace(t, "ws-ext", "a.sql", "notes.md", "a.sql.metadata.json")

	if names := queryNames(t, g, FileQuery{Extensions: []string{".sql"}}); len(names) != 1 || names[0] != "a.sql" {
		t.Errorf("expected only the .sql file, got %v", names)
	}

	// The sidecar is not a file of its own, whatever the query asks for.
	for _, name := range queryNames(t, g, FileQuery{}) {
		if name == "a.sql.metadata.json" {
			t.Errorf("a sidecar should never be a result")
		}
	}
}

func TestFindFiles_CarriesTheDatabasesAFileIsBoundTo(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	g, fsCtx := queryWorkspace(t, "ws-meta", "unopened/query.sql")
	sidecar := filepath.Join(fsCtx.WorkspaceRoot, "unopened", "query.sql.metadata.json")
	if err := os.WriteFile(sidecar, []byte(`{"databases":[{"id":"db-1","name":"DB1"}]}`), 0o600); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	files, err := g.FindFiles(t.Context(), FileQuery{Pattern: "query"})
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(files[0].Databases) != 1 || files[0].Databases[0].ID != "db-1" {
		t.Errorf("opening a result from a picker must bind its databases: %+v", files[0])
	}
	if files[0].FolderID != fsCtx.URI("unopened") {
		t.Errorf("folder_id mismatch: %q", files[0].FolderID)
	}
}

func TestFindFiles_StopsWhenTheCallerCancels(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	g, _ := queryWorkspace(t, "ws-cancel", "a.sql", "b.sql")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := g.FindFiles(ctx, FileQuery{}); err == nil {
		t.Error("expected a cancelled query to report it, so a superseded keystroke costs nothing")
	}
}
