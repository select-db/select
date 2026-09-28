package cellar

import (
	"errors"
	"io/fs"
	"net/http"
	"os"

	"github.com/selectDb/dialect/engine/connect"
)

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
