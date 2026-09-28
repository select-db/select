package cellar

import (
	"net/http"
)

// DeleteHandler removes a database from the disk and from its replica.
func DeleteHandler(databases *Databases) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := databases.remove(r.Context(), GetGrant(r).DatasourceID); err != nil {
			writeLifecycleError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
