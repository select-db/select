package cellar

import (
	"encoding/json"
	"net/http"
)

// StoredDatabase is one entry of GET /datasources, the list the reconciler checks.
type StoredDatabase struct {
	ID        string `json:"id"`
	SizeBytes int64  `json:"size_bytes"`
}

// InventoryHandler lists every database on this cellar's disk, with its size.
func InventoryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stored := []StoredDatabase{}
		databases.mu.Lock()
		for id, database := range databases.onDisk {
			if size, err := databaseSize(database.path); err == nil {
				stored = append(stored, StoredDatabase{ID: id, SizeBytes: size})
			}
		}
		databases.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stored)
	}
}
