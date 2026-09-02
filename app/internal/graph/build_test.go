package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"selectDb/internal/server"
	"selectDb/internal/utils"
)

// withTempAppDataDir configures APP_ENV and HOME so that GetAppDataDir points
// into a test-specific temporary directory, and sets a current server so that
// WorkspaceRootPath works. It returns the resolved app root and a restore function.
func withTempAppDataDir(t *testing.T) (string, func()) {
	t.Helper()

	oldHome := os.Getenv("HOME")
	oldEnv := os.Getenv("APP_ENV")

	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	if err := os.Setenv("APP_ENV", "test-build-graph"); err != nil {
		t.Fatalf("set APP_ENV: %v", err)
	}

	appRoot, err := utils.GetAppDataDir()
	if err != nil {
		t.Fatalf("GetAppDataDir: %v", err)
	}

	// Set current server so WorkspaceRootPath works (appRoot/<domain>/workspaces/...)
	const testDomain = "test.local"
	serverDir := filepath.Join(appRoot, server.DomainToFolderName(testDomain))
	if err := os.MkdirAll(serverDir, 0o700); err != nil {
		t.Fatalf("create server dir: %v", err)
	}
	if err := server.WriteCurrentDomain(testDomain); err != nil {
		t.Fatalf("write current server: %v", err)
	}

	restore := func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("APP_ENV", oldEnv)
	}

	return appRoot, restore
}

// TestBuildWorkspaceGraphFromFS_SimpleTree verifies that the filesystem-based
// graph builder correctly discovers folders, files and db instances under the
// workspace root.
func TestBuildWorkspaceGraphFromFS_SimpleTree(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	const workspaceID = "ws-1"
	rootURI := fmt.Sprintf("selectdb://workspaces/%s", workspaceID)

	serverRoot, err := server.CurrentServerRoot()
	if err != nil {
		t.Fatalf("CurrentServerRoot: %v", err)
	}
	workspaceRoot := filepath.Join(serverRoot, "workspaces", workspaceID)

	// Layout:
	//   workspaces/ws-1/
	//     file-root.sql
	//     folder-file-1/file-child.sql
	//     folder-db-1/db1/db.config.json
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "folder-file-1"), 0o700); err != nil {
		t.Fatalf("mkdir folder-file-1: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "folder-db-1", "db1"), 0o700); err != nil {
		t.Fatalf("mkdir folder-db-1/db1: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspaceRoot, "file-root.sql"), []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatalf("write file-root.sql: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "folder-file-1", "file-child.sql"), []byte("SELECT 2;"), 0o600); err != nil {
		t.Fatalf("write file-child.sql: %v", err)
	}

	dbConfig := `{
  "version": 1,
  "id": "db-1",
  "name": "DB1",
  "db_type": "sqlite",
  "dsn": "file:db1.sqlite",
  "workspace_id": "ws-1"
}`
	if err := os.WriteFile(filepath.Join(workspaceRoot, "folder-db-1", "db1", "db.config.json"), []byte(dbConfig), 0o600); err != nil {
		t.Fatalf("write db.config.json: %v", err)
	}
	// Files inside the DB instance folder must appear in db.Files.
	if err := os.WriteFile(filepath.Join(workspaceRoot, "folder-db-1", "db1", "schema.sql"), []byte("-- schema"), 0o600); err != nil {
		t.Fatalf("write schema.sql in db folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "folder-db-1", "db1", "init.sql"), nil, 0o600); err != nil {
		t.Fatalf("write init.sql in db folder: %v", err)
	}

	// Seed a minimal workspace graph and invoke the filesystem-based builder.
	g := &Graph{
		WorkspaceGraph: &WorkspaceNode{
			ID:          workspaceID,
			Type:        "workspace",
			Name:        "MyWorkspace",
			User:        &UserNode{ID: "user-1", Type: "user", Name: "Alice"},
			Folders:     []*FolderNode{},
			DBInstances: []*DBInstanceNode{},
		},
	}

	fsCtx := NewWorkspaceFSFromRoot(workspaceID, workspaceRoot)
	if err := g.buildWorkspaceGraphFromFS(fsCtx); err != nil {
		t.Fatalf("buildWorkspaceGraphFromFS failed: %v", err)
	}

	ws := g.WorkspaceGraph

	if ws.ID != workspaceID {
		t.Fatalf("workspace ID mismatch: got %q want %q", ws.ID, workspaceID)
	}

	if len(ws.Folders) != 1 {
		t.Fatalf("expected 1 root folder, got %d", len(ws.Folders))
	}

	root := ws.Folders[0]
	if root.ID != rootURI {
		t.Errorf("root folder ID mismatch: got %q want %q", root.ID, rootURI)
	}

	// Root should contain the file at workspace root and the two subfolders.
	if len(root.Files) != 1 || root.Files[0].Name != "file-root.sql" {
		t.Errorf("root files mismatch: %+v", root.Files)
	}

	if len(root.Folders) != 2 {
		t.Fatalf("expected 2 subfolders under root, got %d", len(root.Folders))
	}

	// Collect subfolder IDs for easier assertions.
	subIDs := map[string]*FolderNode{}
	for _, f := range root.Folders {
		subIDs[f.ID] = f
	}

	// A folder below the root is part of the skeleton, but its files are read
	// when it is opened, not by the build.
	folderFileURI := rootURI + "/folder-file-1"
	if ff, ok := subIDs[folderFileURI]; !ok {
		t.Errorf("missing folder %q under root", folderFileURI)
	} else {
		if ff.Resolved {
			t.Errorf("folder-file-1 should not be resolved by the build")
		}
		if len(ff.Files) != 0 {
			t.Errorf("folder-file-1 should hold no files before it is opened: %+v", ff.Files)
		}

		if _, err := g.ResolveFolder(folderFileURI); err != nil {
			t.Fatalf("ResolveFolder failed: %v", err)
		}
		if !ff.Resolved {
			t.Errorf("folder-file-1 should be resolved after opening it")
		}
		if len(ff.Files) != 1 || ff.Files[0].Name != "file-child.sql" {
			t.Errorf("folder-file-1 contents mismatch: %+v", ff.Files)
		}

		// Opening it again must not duplicate what it already holds.
		if _, err := g.ResolveFolder(folderFileURI); err != nil {
			t.Fatalf("second ResolveFolder failed: %v", err)
		}
		if len(ff.Files) != 1 {
			t.Errorf("re-opening folder-file-1 duplicated its files: %+v", ff.Files)
		}
	}

	folderDbURI := rootURI + "/folder-db-1"
	if _, ok := subIDs[folderDbURI]; !ok {
		t.Errorf("missing folder %q under root", folderDbURI)
	}

	// DB instances discovered from db.config.json should be attached to the
	// parent folder and also listed at workspace level.
	if len(ws.DBInstances) != 1 {
		t.Fatalf("expected 1 db instance at workspace level, got %d", len(ws.DBInstances))
	}
	db := ws.DBInstances[0]

	expectedDbURI := rootURI + "/folder-db-1/db1"
	// Name and ID come from db.config.json
	if db.URI != expectedDbURI || db.Name != "DB1" || db.ID != "db-1" || db.DBType != "sqlite" {
		t.Errorf("db instance mismatch: %+v", db)
	}

	if db.FolderID != folderDbURI {
		t.Errorf("db instance folderID mismatch: got %q want %q", db.FolderID, folderDbURI)
	}

	// Files inside the DB instance folder must be attached to the DB node.
	if len(db.Files) != 2 {
		t.Errorf("expected 2 files in db instance (schema.sql, init.sql), got %d: %+v", len(db.Files), db.Files)
	} else {
		names := map[string]bool{}
		for _, f := range db.Files {
			names[f.Name] = true
		}
		if !names["schema.sql"] || !names["init.sql"] {
			t.Errorf("db instance files missing schema.sql or init.sql: %+v", db.Files)
		}
	}
}

// A picker searches the whole workspace by name, so it needs every file, not
// just the ones in folders that have been opened.
func TestListWorkspaceFiles_SeesUnopenedFolders(t *testing.T) {
	_, restore := withTempAppDataDir(t)
	defer restore()

	const workspaceID = "ws-list"
	serverRoot, err := server.CurrentServerRoot()
	if err != nil {
		t.Fatalf("CurrentServerRoot: %v", err)
	}
	workspaceRoot := filepath.Join(serverRoot, "workspaces", workspaceID)

	if err := os.MkdirAll(filepath.Join(workspaceRoot, "unopened", "deeper"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, rel := range []string{"root.sql", "unopened/child.sql", "unopened/deeper/deep.sql"} {
		if err := os.WriteFile(filepath.Join(workspaceRoot, filepath.FromSlash(rel)), []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	// Sidecars and other internal files are not user-facing files.
	if err := os.WriteFile(filepath.Join(workspaceRoot, "root.sql.metadata.json"), []byte(`{"databases":[{"id":"db-1","name":"DB1"}]}`), 0o600); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	g := &Graph{WorkspaceGraph: &WorkspaceNode{ID: workspaceID, Type: "workspace"}}
	fsCtx := NewWorkspaceFSFromRoot(workspaceID, workspaceRoot)
	if err := g.buildWorkspaceGraphFromFS(fsCtx); err != nil {
		t.Fatalf("build: %v", err)
	}

	files, err := g.ListWorkspaceFiles()
	if err != nil {
		t.Fatalf("ListWorkspaceFiles: %v", err)
	}

	byURI := map[string]*FileNode{}
	for _, f := range files {
		byURI[f.URI] = f
	}
	if len(byURI) != 3 {
		t.Fatalf("expected 3 files, got %d: %+v", len(byURI), byURI)
	}
	for _, rel := range []string{"root.sql", "unopened/child.sql", "unopened/deeper/deep.sql"} {
		if byURI[fsCtx.URI(rel)] == nil {
			t.Errorf("missing %s from the listing", rel)
		}
	}

	// The databases a file is bound to travel with it, so opening one from a
	// picker binds it the same way opening it from the tree does.
	root := byURI[fsCtx.URI("root.sql")]
	if root == nil || len(root.Databases) != 1 || root.Databases[0].ID != "db-1" {
		t.Errorf("listing dropped the file's databases: %+v", root)
	}

	// Listing must not pull the walked folders into the graph.
	if folder, _ := g.lookup(fsCtx.URI("unopened")).(*FolderNode); folder == nil || folder.Resolved {
		t.Errorf("listing should leave unopened folders alone, got %+v", folder)
	}
}
