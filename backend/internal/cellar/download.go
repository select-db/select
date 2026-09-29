package cellar

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// DownloadHandler sends a consistent copy of the database, taken with VACUUM
// INTO so writes may go on while it streams.
func DownloadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sourcePath, err := databases.use(r.Context(), GetGrant(r).DatasourceID)
		if err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		tempPath := filepath.Join(databases.dir, ".tmp-"+uuid.NewString()+".db")
		defer func() { _ = os.Remove(tempPath) }()
		if err := execTrusted(r.Context(), sourcePath, "rw", "VACUUM INTO ?", tempPath); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.sqlite3")
		http.ServeFile(w, r, tempPath)
	}
}
