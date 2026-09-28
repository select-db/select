package cellar

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// execTrusted runs one statement on a connection of its own, outside the
// isolation rules user statements run under, and closes it.
func execTrusted(ctx context.Context, path, mode, statement string, args ...any) error {
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=" + mode + "&_busy_timeout=5000"}).String()
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_, err = conn.ExecContext(ctx, statement, args...)
	return err
}

// databasePath is the file of database id. The id must be a uuid, so it cannot
// name a file outside dir.
func databasePath(dir, id string) (string, error) {
	if _, err := uuid.Parse(id); err != nil {
		return "", fmt.Errorf("datasource id %q is not a uuid", id)
	}
	return filepath.Join(dir, id+".db"), nil
}

// existingDatabasePath is databasePath for a database that must already exist.
func existingDatabasePath(dir, id string) (string, error) {
	path, err := databasePath(dir, id)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", errNotFound
	}
	return path, nil
}

// databaseSize counts the WAL too: until a checkpoint, recent writes live there.
func databaseSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	size := info.Size()
	if wal, err := os.Stat(path + "-wal"); err == nil {
		size += wal.Size()
	}
	return size, nil
}
