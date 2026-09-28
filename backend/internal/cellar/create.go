package cellar

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the body of PUT /datasources/{id}: an empty database, or a
// copy of SourceID, as it was at PointInTime when set (RFC 3339).
type CreateRequest struct {
	SourceID    string `json:"from,omitempty"`
	PointInTime string `json:"at,omitempty"`
}

// CreateHandler writes a new database, empty or copied, under a temporary name
// and renames it into place: a failed copy never leaves a half database.
func CreateHandler(databases *Databases) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		id := GetGrant(r).DatasourceID
		if err := createDatabase(r.Context(), databases, id, req); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		size, _ := databaseSize(filepath.Join(databases.dir, id+".db"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(StoredDatabase{ID: id, SizeBytes: size})
	}
}

func createDatabase(ctx context.Context, databases *Databases, id string, req CreateRequest) error {
	// Use wakes the id when it is cold, so an id kept only in the replica is taken too.
	_, err := databases.Use(ctx, id)
	switch {
	case err == nil:
		return errAlreadyExists
	case err != errNotFound:
		return err
	}
	tempPath := filepath.Join(databases.dir, ".tmp-"+uuid.NewString()+".db")
	defer func() { _ = os.Remove(tempPath) }()

	switch {
	case req.PointInTime != "":
		at, err := time.Parse(time.RFC3339, req.PointInTime)
		if err != nil {
			return err
		}
		if err := databases.RestoreAt(ctx, req.SourceID, at, tempPath); err != nil {
			return err
		}
	case req.SourceID != "":
		sourcePath, err := databases.Use(ctx, req.SourceID)
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
	return databases.add(id, tempPath)
}
