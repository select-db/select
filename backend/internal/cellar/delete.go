package cellar

import (
	"net/http"
)

// DeleteHandler removes a database from the cellar's disk and from the bucket.
func DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := databases.remove(r.Context(), GetGrant(r).DatasourceID); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
