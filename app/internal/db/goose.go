package db

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"selectDb/internal/cmd/scripts"
)

// GooseCommand is a migration command applied to one server's database.
type GooseCommand string

const (
	GooseUp     GooseCommand = "up"
	GooseDown   GooseCommand = "down"
	GooseReset  GooseCommand = "reset"
	GooseStatus GooseCommand = "status"
)

// RunGooseAt applies a migration command to the database at dbPath, from the
// same embedded migrations the app runs at startup. Every server has its own
// SQLite file, so the caller says which one.
//
// Migrations are embedded, not read off disk, so a packaged build and a
// developer's checkout run byte-identical SQL.
//
// schemaOut, when set, is where the resulting schema is written after a
// command that changes it. sqlc generates the app's queries from that file
// (internal/sqlc.yaml names it), so it has to follow the migrations or codegen
// drifts from the database. Only the development command passes a path: the
// shipped binary migrates a user's database and has no source tree to update.
func RunGooseAt(dbPath string, cmd GooseCommand, schemaOut string) error {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer func() { _ = conn.Close() }()

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}

	switch cmd {
	case GooseUp:
		err = goose.Up(conn, migrationsDir)
	case GooseDown:
		err = goose.Down(conn, migrationsDir)
	case GooseReset:
		err = goose.Reset(conn, migrationsDir)
	case GooseStatus:
		return goose.Status(conn, migrationsDir)
	default:
		return fmt.Errorf("unknown migration command %q (want up, down, reset or status)", cmd)
	}
	if err != nil || schemaOut == "" {
		return err
	}
	if err := scripts.DumpSchema(conn, schemaOut); err != nil {
		return fmt.Errorf("dump schema to %s: %w", schemaOut, err)
	}
	return nil
}

// NewMigration scaffolds an empty migration in the source tree. It writes a
// file rather than touching a database, so it takes the directory on disk and
// not one of the embedded copies.
func NewMigration(dir, name string) error {
	goose.SetBaseFS(nil)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Create(nil, dir, name, "sql")
}
