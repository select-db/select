package cellar

import (
	"net/http"
)

// DeleteHandler removes a database from the disk and from the bucket.
func DeleteHandler(databases *Databases) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := databases.remove(r.Context(), GetGrant(r).DatasourceID); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
