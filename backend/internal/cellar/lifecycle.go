package cellar

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/selectDb/dialect/engine/connect"
)

// CreateRequest is the body of PUT /datasources/{id}: an empty database, or a
// copy of SourceID. PointInTime needs the replica the bucket keeps.
type CreateRequest struct {
	SourceID    string `json:"from,omitempty"`
	PointInTime string `json:"at,omitempty"`
}

// StoredDatabase is one entry of GET /datasources, the list the reconciler checks.
type StoredDatabase struct {
	ID        string `json:"id"`
	SizeBytes int64  `json:"size_bytes"`
}

var (
	errAlreadyExists       = &arrowstream.Error{Code: CodeSQLError, Message: "a managed database with this id already exists"}
	errNotFound            = &arrowstream.Error{Code: CodeSQLError, Message: "managed database not found"}
	errPointInTimeDisabled = &arrowstream.Error{Code: CodeDisabled, Message: "point-in-time fork is not available yet"}
)

// CreateHandler writes a new database, empty or copied, under a temporary name
// and renames it into place: a failed copy never leaves a half database.
func CreateHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		id := GetGrant(r).DatasourceID
		if err := createDatabase(r.Context(), dir, id, req); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		size, _ := databaseSize(filepath.Join(dir, id+".db"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(StoredDatabase{ID: id, SizeBytes: size})
	}
}

func createDatabase(ctx context.Context, dir, id string, req CreateRequest) error {
	if req.PointInTime != "" {
		return errPointInTimeDisabled
	}
	targetPath, err := databasePath(dir, id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(targetPath); err == nil {
		return errAlreadyExists
	}
	tempPath := filepath.Join(dir, ".tmp-"+uuid.NewString()+".db")
	defer func() { _ = os.Remove(tempPath) }()

	if req.SourceID != "" {
		sourcePath, err := existingDatabasePath(dir, req.SourceID)
		if err != nil {
			return err
		}
		if err := execTrusted(ctx, sourcePath, "rw", "VACUUM INTO ?", tempPath); err != nil {
			return err
		}
	}
	// WAL is a property of the file: set once here, every open keeps it.
	if err := execTrusted(ctx, tempPath, "rwc", "PRAGMA journal_mode = WAL"); err != nil {
		return err
	}
	return os.Rename(tempPath, targetPath)
}

// DownloadHandler sends a consistent copy of the database, taken with VACUUM
// INTO so writes may go on while it streams.
func DownloadHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sourcePath, err := existingDatabasePath(dir, GetGrant(r).DatasourceID)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		tempPath := filepath.Join(dir, ".tmp-"+uuid.NewString()+".db")
		defer func() { _ = os.Remove(tempPath) }()
		if err := execTrusted(r.Context(), sourcePath, "rw", "VACUUM INTO ?", tempPath); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.sqlite3")
		http.ServeFile(w, r, tempPath)
	}
}

// DeleteHandler removes a database and its WAL files. Its pooled connections
// close after their grace period; nothing reaches them once the file is gone.
func DeleteHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetPath, err := databasePath(dir, GetGrant(r).DatasourceID)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		connect.DeleteConnsByAddr(targetPath)
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := os.Remove(targetPath + suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
				writeLifecycleError(w, r, err)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// InventoryHandler lists every database this cellar holds, with its size.
func InventoryHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		databases := []StoredDatabase{}
		for _, entry := range entries {
			// Only <uuid>.db is a database: this skips WAL files and temporary copies.
			id, isDatabase := strings.CutSuffix(entry.Name(), ".db")
			if _, err := uuid.Parse(id); !isDatabase || err != nil {
				continue
			}
			size, err := databaseSize(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			databases = append(databases, StoredDatabase{ID: id, SizeBytes: size})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(databases)
	}
}

// writeLifecycleError answers with the failure's code and message as JSON; any
// other error is classified as a statement's would be.
func writeLifecycleError(w http.ResponseWriter, r *http.Request, err error) {
	var coded *arrowstream.Error
	if !errors.As(err, &coded) {
		coded = classify(r.Context(), err, GetGrant(r))
	}
	status := http.StatusBadRequest
	switch {
	case coded == errAlreadyExists:
		status = http.StatusConflict
	case coded == errNotFound:
		status = http.StatusNotFound
	case coded == errPointInTimeDisabled:
		status = http.StatusNotImplemented
	case coded.Code == CodeInternal:
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(coded)
}

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
