package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"selectDb/internal/db/generated"
)

// newTestGraphWithWorkspace returns a graph holding an empty workspace, and the
// WorkspaceFS pointing at that workspace's root on disk.
//
// The workspace is named for the test that asked for it. These tests care about
// directories that should *not* be there, so two of them sharing a workspace
// root is the difference between passing and failing.
func newTestGraphWithWorkspace(t *testing.T) (*Graph, *WorkspaceFS) {
	t.Helper()

	_, restore := withTempAppDataDir(t)
	t.Cleanup(restore)

	workspaceID := strings.ReplaceAll(t.Name(), "/", "-")

	fsCtx, err := NewWorkspaceFS(workspaceID)
	if err != nil {
		t.Fatalf("NewWorkspaceFS: %v", err)
	}
	if err := os.MkdirAll(fsCtx.WorkspaceRoot, 0o700); err != nil {
		t.Fatalf("mkdir workspace root: %v", err)
	}

	g := New(&generated.Queries{})
	g.WorkspaceGraph = &WorkspaceNode{
		ID:          workspaceID,
		Type:        "workspace",
		Name:        "workspace",
		Folders:     []*FolderNode{},
		DBInstances: []*DBInstanceNode{},
	}
	g.ensureIndex()

	return g, fsCtx
}

func TestAvailableFolderName(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"analytics", "analytics-2", "Reports"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.sql"), nil, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tests := []struct {
		why  string
		name string
		want string
	}{
		{
			why:  "a free name is used as it is",
			name: "warehouse",
			want: "warehouse",
		},
		{
			why:  "a taken one is numbered from two",
			name: "analytics",
			want: "analytics-3", // analytics-2 is taken as well
		},
		{
			why:  "case does not free a name up: macOS and Windows would collide",
			name: "REPORTS",
			want: "REPORTS-2",
		},
		{
			why:  "a file holds a name as surely as a directory does",
			name: "notes.sql",
			want: "notes.sql-2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.why, func(t *testing.T) {
			got, err := AvailableFolderName(dir, tc.name)
			if err != nil {
				t.Fatalf("AvailableFolderName: %v", err)
			}
			if got != tc.want {
				t.Errorf("AvailableFolderName(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestAvailableFolderNameInAFolderThatIsNotThereYet(t *testing.T) {
	// A database can be made in a folder that is itself still on its way. An
	// empty directory and a missing one hold the same number of names.
	got, err := AvailableFolderName(filepath.Join(t.TempDir(), "not-yet"), "analytics")
	if err != nil {
		t.Fatalf("AvailableFolderName: %v", err)
	}
	if got != "analytics" {
		t.Errorf("got %q, want %q", got, "analytics")
	}
}

func TestCreateDatabaseWritesADirectoryAndItsConfig(t *testing.T) {
	g, fsCtx := newTestGraphWithWorkspace(t)

	created, err := g.CreateDatabase(CreateDatabaseParams{
		FolderURI: fsCtx.RootURI,
		Name:      "Prod Analytics",
	})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}

	if created.Name != "Prod Analytics" {
		t.Errorf("name = %q, want %q", created.Name, "Prod Analytics")
	}
	if want := fsCtx.URI("Prod Analytics"); created.URI != want {
		t.Errorf("uri = %q, want %q", created.URI, want)
	}

	dbPath := filepath.Join(fsCtx.WorkspaceRoot, "Prod Analytics")
	if !CheckIsDBInstance(dbPath) {
		t.Fatalf("no %s in %s", DBConfigFileName, dbPath)
	}

	cfg, err := ReadFSDBConfig(filepath.Join(dbPath, DBConfigFileName))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.ID != created.ID {
		t.Errorf("config id = %q, want %q", cfg.ID, created.ID)
	}
	if cfg.DbType != defaultNewDatabaseType {
		t.Errorf("config db_type = %q, want %q", cfg.DbType, defaultNewDatabaseType)
	}
}

func TestCreateDatabaseCleansTheNameItIsGiven(t *testing.T) {
	g, fsCtx := newTestGraphWithWorkspace(t)

	// A separator would make two path components out of one name.
	created, err := g.CreateDatabase(CreateDatabaseParams{
		FolderURI: fsCtx.RootURI,
		Name:      "sales/eu",
	})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}

	if created.Name != "sales-eu" {
		t.Errorf("name = %q, want %q", created.Name, "sales-eu")
	}
	if !CheckIsDBInstance(filepath.Join(fsCtx.WorkspaceRoot, "sales-eu")) {
		t.Errorf("no database directory at sales-eu")
	}
}

func TestCreateDatabaseDoesNotTakeASiblingsName(t *testing.T) {
	g, fsCtx := newTestGraphWithWorkspace(t)

	first, err := g.CreateDatabase(CreateDatabaseParams{FolderURI: fsCtx.RootURI, Name: "analytics"})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
	second, err := g.CreateDatabase(CreateDatabaseParams{FolderURI: fsCtx.RootURI, Name: "analytics"})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}

	if first.Name != "analytics" || second.Name != "analytics-2" {
		t.Errorf("names = %q, %q; want %q, %q", first.Name, second.Name, "analytics", "analytics-2")
	}
	if first.ID == second.ID {
		t.Errorf("both databases got the same id %q", first.ID)
	}
}

func TestCreateDatabaseRefusesAFolderOutsideTheWorkspace(t *testing.T) {
	g, _ := newTestGraphWithWorkspace(t)

	if _, err := g.CreateDatabase(CreateDatabaseParams{
		FolderURI: "selectdb://workspaces/somewhere-else/folder",
		Name:      "analytics",
	}); err == nil {
		t.Fatal("expected an error for a folder outside the workspace")
	}
}
