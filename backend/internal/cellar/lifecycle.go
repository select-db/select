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

// Create is the body of PUT /datasources/{id}: an empty database, or a copy of
// From. At, a point in time of From, needs the replica the bucket keeps.
type Create struct {
	From string `json:"from,omitempty"`
	At   string `json:"at,omitempty"`
}

// Stored is one database of GET /datasources, the list the reconciler checks.
type Stored struct {
	ID        string `json:"id"`
	SizeBytes int64  `json:"size_bytes"`
}

var (
	errExists  = &arrowstream.Error{Code: CodeSQLError, Message: "a managed database with this id already exists"}
	errMissing = &arrowstream.Error{Code: CodeSQLError, Message: "managed database not found"}
	errNoPITR  = &arrowstream.Error{Code: CodeDisabled, Message: "point-in-time fork is not available yet"}
)

// CreateHandler writes a new database, empty or copied, under a temporary name
// and renames it into place: a failed copy never leaves a half database.
func CreateHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Create
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		id := GetGrant(r).DatasourceID
		err := create(r.Context(), dir, id, req)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		size, _ := fileSize(filepath.Join(dir, id+".db"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Stored{ID: id, SizeBytes: size})
	}
}

func create(ctx context.Context, dir, id string, req Create) error {
	if req.At != "" {
		return errNoPITR
	}
	target, err := checkedPath(dir, id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return errExists
	}
	tmp := filepath.Join(dir, ".tmp-"+uuid.NewString()+".db")
	defer func() { _ = os.Remove(tmp) }()

	if req.From != "" {
		source, err := existingPath(dir, req.From)
		if err != nil {
			return err
		}
		if err := trusted(ctx, source, "rw", "VACUUM INTO ?", tmp); err != nil {
			return err
		}
	}
	// WAL is a property of the file: set once here, every open keeps it.
	if err := trusted(ctx, tmp, "rwc", "PRAGMA journal_mode = WAL"); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// DownloadHandler sends a consistent copy of the database, taken with VACUUM
// INTO so writes may go on while it streams.
func DownloadHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := GetGrant(r).DatasourceID
		source, err := existingPath(dir, id)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		tmp := filepath.Join(dir, ".tmp-"+uuid.NewString()+".db")
		defer func() { _ = os.Remove(tmp) }()
		if err := trusted(r.Context(), source, "rw", "VACUUM INTO ?", tmp); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.sqlite3")
		http.ServeFile(w, r, tmp)
	}
}

// DeleteHandler removes a database and its WAL files. Its pooled connections
// close after their grace period; nothing reaches them once the file is gone.
func DeleteHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := GetGrant(r).DatasourceID
		target, err := checkedPath(dir, id)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		connect.DeleteConnsByAddr(target)
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := os.Remove(target + suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
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
		stored := []Stored{}
		for _, e := range entries {
			id, ok := strings.CutSuffix(e.Name(), ".db")
			if _, err := uuid.Parse(id); !ok || err != nil {
				continue
			}
			size, err := fileSize(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			stored = append(stored, Stored{ID: id, SizeBytes: size})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stored)
	}
}

// writeLifecycleError answers with the failure's code and message as JSON; an
// unplaced failure is classified as a statement's would be.
func writeLifecycleError(w http.ResponseWriter, r *http.Request, err error) {
	var coded *arrowstream.Error
	if !errors.As(err, &coded) {
		coded = classify(r.Context(), err, GetGrant(r))
	}
	status := http.StatusBadRequest
	switch {
	case coded == errExists:
		status = http.StatusConflict
	case coded == errMissing:
		status = http.StatusNotFound
	case coded == errNoPITR:
		status = http.StatusNotImplemented
	case coded.Code == CodeInternal:
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(coded)
}

// trusted runs one statement on a connection of its own, outside the
// isolation rules user statements run under, and closes it.
func trusted(ctx context.Context, file, mode, statement string, args ...any) error {
	dsn := (&url.URL{Scheme: "file", Path: file, RawQuery: "mode=" + mode + "&_busy_timeout=5000"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.ExecContext(ctx, statement, args...)
	return err
}

// checkedPath is the file of database id; the id must be a uuid, so it names
// nothing outside dir.
func checkedPath(dir, id string) (string, error) {
	if _, err := uuid.Parse(id); err != nil {
		return "", fmt.Errorf("datasource id %q is not a uuid", id)
	}
	return filepath.Join(dir, id+".db"), nil
}

// existingPath is checkedPath for a database that must already exist.
func existingPath(dir, id string) (string, error) {
	p, err := checkedPath(dir, id)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err != nil {
		return "", errMissing
	}
	return p, nil
}

// fileSize counts the WAL too: until a checkpoint, recent writes live there.
func fileSize(p string) (int64, error) {
	info, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	size := info.Size()
	if wal, err := os.Stat(p + "-wal"); err == nil {
		size += wal.Size()
	}
	return size, nil
}
