package cellar

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// StoredDatabase is one entry of GET /datasources, the list the reconciler checks.
type StoredDatabase struct {
	ID        string `json:"id"`
	SizeBytes int64  `json:"size_bytes"`
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
