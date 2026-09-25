package datasource

import (
	"net/http"

	"backend/internal/middlewares"

	"github.com/selectDb/dialect/engine/transport"
)

func SchemaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		noCache := r.URL.Query().Get("no_cache") == "true"
		workspaceID := middlewares.MemberWorkspaceID(r)

		o, err := Open(r, id, workspaceID)
		if err != nil {
			OpenError(w, err, "datasource schema", workspaceID, id)
			return
		}
		meta, err := o.Metadata(r.Context(), noCache)
		if err != nil {
			OpenError(w, err, "datasource schema", workspaceID, id)
			return
		}
		transport.WriteZstdJSON(w, meta)
	}
}
