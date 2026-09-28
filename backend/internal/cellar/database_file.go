package cellar

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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

// databaseID is the id of the database file named fileName, if it is one:
// temporary copies, WAL files and Litestream's folders are not.
func databaseID(fileName string) (string, bool) {
	id, isDatabase := strings.CutSuffix(fileName, ".db")
	_, err := uuid.Parse(id)
	return id, isDatabase && err == nil
}

// removeDatabaseFiles removes database id and everything next to it: its WAL
// files and Litestream's working folder.
func removeDatabaseFiles(dir, id string) error {
	path := filepath.Join(dir, id+".db")
	for _, file := range []string{path, path + "-wal", path + "-shm", filepath.Join(dir, "."+id+".db-litestream")} {
		if err := os.RemoveAll(file); err != nil {
			return err
		}
	}
	return nil
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
