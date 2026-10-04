package cellar

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// StoredDatabase is one entry of GET /datasources, the list the reconciler checks.
type StoredDatabase struct {
	ID        string `json:"id"`
	SizeBytes int64  `json:"size_bytes"`
	// ModifiedAt is the last write the cellar saw: the file on its disk, or, for
	// a cold database, its newest object in the bucket.
	ModifiedAt time.Time `json:"modified_at"`
	// Cold is a database that lives only in the bucket.
	Cold bool `json:"cold"`
}

// InventoryHandler lists every database of this cellar: the ones on its disk,
// and the cold ones that live only in its bucket. A bucket it cannot list is an
// error, never a short list: the reconciler purges what the list does not hold
// back.
func InventoryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stored, err := databases.inventory(r.Context())
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stored)
	}
}

// inventory is the disk first, then what only the bucket holds.
func (databases *Databases) inventory(ctx context.Context) ([]StoredDatabase, error) {
	inBucket, err := databases.listBucket(ctx)
	if err != nil {
		return nil, err
	}
	stored := []StoredDatabase{}
	databases.mu.Lock()
	for id, database := range databases.onDisk {
		size, err := databaseSize(database.path)
		if err != nil {
			continue
		}
		stored = append(stored, StoredDatabase{ID: id, SizeBytes: size, ModifiedAt: modifiedAt(database.path)})
		delete(inBucket, id)
	}
	databases.mu.Unlock()
	for id, newest := range inBucket {
		stored = append(stored, StoredDatabase{ID: id, ModifiedAt: newest, Cold: true})
	}
	return stored, nil
}

// modifiedAt is the newer of a database file and its WAL: a write sits in the
// WAL until the next checkpoint.
func modifiedAt(path string) time.Time {
	var newest time.Time
	for _, name := range []string{path, path + "-wal"} {
		if info, err := os.Stat(name); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}
