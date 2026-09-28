package cellar

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// CreateRequest is the body of PUT /datasources/{id}: an empty database, or a
// copy of SourceID. PointInTime needs the replica the bucket keeps.
type CreateRequest struct {
	SourceID    string `json:"from,omitempty"`
	PointInTime string `json:"at,omitempty"`
}

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
