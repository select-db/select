package cellar

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

// StoredDatabase is one entry of GET /datasources, the list the reconciler checks.
type StoredDatabase struct {
	ID        string `json:"id"`
	SizeBytes int64  `json:"size_bytes"`
}

// InventoryHandler lists every database on this cellar's disk, with its size.
func InventoryHandler(databases *Databases) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := os.ReadDir(databases.dir)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		stored := []StoredDatabase{}
		for _, entry := range entries {
			id, isDatabase := databaseID(entry.Name())
			if !isDatabase {
				continue
			}
			size, err := databaseSize(filepath.Join(databases.dir, entry.Name()))
			if err != nil {
				continue
			}
			stored = append(stored, StoredDatabase{ID: id, SizeBytes: size})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stored)
	}
}
