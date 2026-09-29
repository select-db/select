package cellar

import (
	"context"
	"encoding/json"
	"errors"
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
func CreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		id := GetGrant(r).DatasourceID
		path, err := createDatabase(r.Context(), id, req)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		size, _ := databaseSize(path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(StoredDatabase{ID: id, SizeBytes: size})
	}
}

func createDatabase(ctx context.Context, id string, req CreateRequest) (string, error) {
	// use wakes the id when it is cold, so an id kept only in the bucket is taken too.
	_, err := databases.use(ctx, id)
	switch {
	case err == nil:
		return "", errAlreadyExists
	case !errors.Is(err, errNotFound):
		return "", err
	}
	tempPath := filepath.Join(databases.dir, ".tmp-"+uuid.NewString()+".db")
	defer func() { _ = os.Remove(tempPath) }()

	switch {
	case req.PointInTime != "":
		at, err := time.Parse(time.RFC3339, req.PointInTime)
		if err != nil {
			return "", err
		}
		if err := databases.restoreAt(ctx, req.SourceID, at, tempPath); err != nil {
			return "", err
		}
	case req.SourceID != "":
		sourcePath, err := databases.use(ctx, req.SourceID)
		if err != nil {
			return "", err
		}
		if err := execTrusted(ctx, sourcePath, "rw", "VACUUM INTO ?", tempPath); err != nil {
			return "", err
		}
	}
	// WAL is a property of the file: set once here, every open keeps it.
	if err := execTrusted(ctx, tempPath, "rwc", "PRAGMA journal_mode = WAL"); err != nil {
		return "", err
	}
	return databases.add(id, tempPath)
}
