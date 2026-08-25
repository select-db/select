// Command e2eseed writes the state an e2e run needs into a throwaway data
// directory: a server with a migrated database, a user, a workspace that user
// is currently in, and a couple of files in it.
//
// It exists because the app shows a login screen until it finds all of that,
// which caps the suite at "it boots". Seeding through the app's own migrations
// and queries means the fixture cannot drift from the real schema.
//
// Authentication is not part of it. The tokens live in the OS keyring, which a
// headless runner does not have, and the only thing that reads them is
// System.CheckForLogin — everything else reads the database. So the suite puts
// the app into its signed-in state by emitting the `login` event the frontend
// already listens for, and this seeds what that state then reads.
//
// Test-only: nothing in the shipped binary imports it.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	sqlite "modernc.org/sqlite"

	"selectDb/internal/db"
	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/graph"
	"selectDb/internal/server"
)

// Fixed so specs can name what they expect to see.
const (
	UserID        = "e2e-user"
	UserName      = "E2E"
	WorkspaceID   = "e2e-workspace"
	WorkspaceName = "E2E Workspace"
)

// The workspace as it looks on disk. Paths are relative to the workspace root.
var workspaceFiles = map[string]string{
	"queries/hello.sql":    "SELECT 1 AS hello;\n",
	"queries/nested/b.sql": "SELECT 2 AS two;\n",
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <data-dir>", filepath.Base(os.Args[0]))
	}

	if err := seed(os.Args[1]); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func seed(dataDir string) error {
	// The app resolves its data directory from the OS config dir, so pointing
	// those at the fixture is what keeps a run out of the real one. Same
	// variables the Playwright config gives the server.
	for _, key := range []string{"XDG_CONFIG_HOME", "HOME"} {
		if err := os.Setenv(key, dataDir); err != nil {
			return err
		}
	}
	if err := os.Setenv("APP_ENV", "dev"); err != nil {
		return err
	}

	// The app registers modernc's driver under this name at startup; the
	// migrations and queries below open it the same way.
	sql.Register("sqlite3", &sqlite.Driver{})

	domain := server.DefaultDomainForEnv()
	if err := server.WriteCurrentDomain(domain); err != nil {
		return fmt.Errorf("select server %q: %w", domain, err)
	}

	dbPath, err := server.ServerDBPath(domain)
	if err != nil {
		return err
	}
	if err := db.RunMigrationsAt(dbPath); err != nil {
		return fmt.Errorf("migrate %s: %w", dbPath, err)
	}

	handle, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer func() { _ = handle.Close() }()

	if err := seedTables(context.Background(), generated.New(handle)); err != nil {
		return err
	}

	return writeWorkspaceFiles()
}

func seedTables(ctx context.Context, queries *generated.Queries) error {
	if _, err := queries.UpsertUser(ctx, generated.UpsertUserParams{
		ID:   UserID,
		Name: db_types.JSONNullString{NullString: sql.NullString{String: UserName, Valid: true}},
	}); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	if _, err := queries.CreateWorkspace(ctx, generated.CreateWorkspaceParams{
		ID:      WorkspaceID,
		Name:    WorkspaceName,
		OwnerID: db_types.JSONNullString{NullString: sql.NullString{String: UserID, Valid: true}},
	}); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	if _, err := queries.CreateWorkspaceToUser(ctx, generated.CreateWorkspaceToUserParams{
		ID:          "e2e-workspace-to-user",
		WorkspaceID: WorkspaceID,
		UserID:      UserID,
	}); err != nil {
		return fmt.Errorf("join user to workspace: %w", err)
	}

	// `current` is what GetCurrentUser and GetCurrentWorkspace select on: it is
	// the difference between a seeded database and a signed-in one.
	if err := queries.UpdateCurrentWorkspaceToUser(ctx, generated.UpdateCurrentWorkspaceToUserParams{
		UserID:      UserID,
		WorkspaceID: WorkspaceID,
	}); err != nil {
		return fmt.Errorf("make the workspace current: %w", err)
	}

	return nil
}

func writeWorkspaceFiles() error {
	root, err := graph.WorkspaceRootPath(WorkspaceID)
	if err != nil {
		return err
	}

	for name, content := range workspaceFiles {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}
