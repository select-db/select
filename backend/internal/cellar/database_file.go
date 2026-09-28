package cellar

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/benbjohnson/litestream"
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

// removeDatabaseFiles removes the database at path and everything next to it:
// its WAL files and Litestream's working folder. The file goes last, so a
// failure never leaves a stale WAL for a restored file to replay.
func removeDatabaseFiles(path string) error {
	metaPath := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+litestream.MetaDirSuffix)
	for _, file := range []string{path + "-wal", path + "-shm", metaPath, path} {
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
