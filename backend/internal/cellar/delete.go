package cellar

import (
	"net/http"
)

// DeleteHandler removes a database from the disk and from its replica. Its
// pooled connections close after their grace period; nothing reaches them once
// the file is gone.
func DeleteHandler(databases *Databases) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := databases.Delete(r.Context(), GetGrant(r).DatasourceID); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
