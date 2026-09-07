package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameDatabase(t *testing.T) {
	g, fsCtx := newTestGraphWithWorkspace(t)

	created, err := g.CreateDatabase(CreateDatabaseParams{FolderURI: fsCtx.RootURI, Name: "analytics"})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}

	renamed, err := g.RenameDatabase(RenameDatabaseParams{URI: created.URI, Name: "warehouse"})
	if err != nil {
		t.Fatalf("RenameDatabase: %v", err)
	}

	if renamed.Name != "warehouse" {
		t.Errorf("name = %q, want %q", renamed.Name, "warehouse")
	}
	if renamed.ID != created.ID {
		t.Errorf("id = %q, want %q — a rename is not a new database", renamed.ID, created.ID)
	}
	if want := fsCtx.URI("warehouse"); renamed.URI != want {
		t.Errorf("uri = %q, want %q", renamed.URI, want)
	}

	if !CheckIsDBInstance(filepath.Join(fsCtx.WorkspaceRoot, "warehouse")) {
		t.Errorf("no database directory at warehouse")
	}
	if _, err := os.Stat(filepath.Join(fsCtx.WorkspaceRoot, "analytics")); !os.IsNotExist(err) {
		t.Errorf("the old directory is still there")
	}
}

func TestRenameDatabaseCleansTheNameItIsGiven(t *testing.T) {
	g, fsCtx := newTestGraphWithWorkspace(t)

	created, err := g.CreateDatabase(CreateDatabaseParams{FolderURI: fsCtx.RootURI, Name: "analytics"})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}

	renamed, err := g.RenameDatabase(RenameDatabaseParams{URI: created.URI, Name: "sales: EU"})
	if err != nil {
		t.Fatalf("RenameDatabase: %v", err)
	}
	if renamed.Name != "sales- EU" {
		t.Errorf("name = %q, want %q", renamed.Name, "sales- EU")
	}
}

func TestRenameDatabaseRefusesANameASiblingHas(t *testing.T) {
	// Numbering it would answer a question the user did not ask: they named
	// this database, they did not ask for the nearest free name.
	g, fsCtx := newTestGraphWithWorkspace(t)

	if _, err := g.CreateDatabase(CreateDatabaseParams{FolderURI: fsCtx.RootURI, Name: "warehouse"}); err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
	created, err := g.CreateDatabase(CreateDatabaseParams{FolderURI: fsCtx.RootURI, Name: "analytics"})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}

	if _, err := g.RenameDatabase(RenameDatabaseParams{URI: created.URI, Name: "warehouse"}); err == nil {
		t.Fatal("expected an error renaming onto a sibling's name")
	}

	// And it is still where it was.
	if !CheckIsDBInstance(filepath.Join(fsCtx.WorkspaceRoot, "analytics")) {
		t.Errorf("the database moved despite the refusal")
	}
}

func TestRenameDatabaseToTheNameItAlreadyHas(t *testing.T) {
	// Its own name is not a name that is taken, so this is a no-op rather than
	// the collision error.
	g, fsCtx := newTestGraphWithWorkspace(t)

	created, err := g.CreateDatabase(CreateDatabaseParams{FolderURI: fsCtx.RootURI, Name: "analytics"})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}

	renamed, err := g.RenameDatabase(RenameDatabaseParams{URI: created.URI, Name: "analytics"})
	if err != nil {
		t.Fatalf("RenameDatabase: %v", err)
	}
	if renamed.Name != "analytics" || renamed.URI != created.URI {
		t.Errorf("got %q at %q, want %q at %q", renamed.Name, renamed.URI, "analytics", created.URI)
	}
}

func TestRenameDatabaseChangingOnlyCase(t *testing.T) {
	// The directory it would collide with is itself. On a case-insensitive
	// filesystem this is the one rename where a taken name is not a collision.
	g, fsCtx := newTestGraphWithWorkspace(t)

	created, err := g.CreateDatabase(CreateDatabaseParams{FolderURI: fsCtx.RootURI, Name: "analytics"})
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}

	renamed, err := g.RenameDatabase(RenameDatabaseParams{URI: created.URI, Name: "Analytics"})
	if err != nil {
		t.Fatalf("RenameDatabase: %v", err)
	}
	if renamed.Name != "Analytics" {
		t.Errorf("name = %q, want %q", renamed.Name, "Analytics")
	}
}

func TestRenameDatabaseRefusesWhatIsNotADatabase(t *testing.T) {
	g, fsCtx := newTestGraphWithWorkspace(t)

	plain := filepath.Join(fsCtx.WorkspaceRoot, "reports")
	if err := os.MkdirAll(plain, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if _, err := g.RenameDatabase(RenameDatabaseParams{
		URI:  fsCtx.URI("reports"),
		Name: "analytics",
	}); err == nil {
		t.Fatal("expected an error renaming a folder that is not a database")
	}
}
