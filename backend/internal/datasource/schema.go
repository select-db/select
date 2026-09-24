package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/middlewares"

	"github.com/klauspost/compress/zstd"
)

var zstdEncoder, _ = zstd.NewWriter(nil)

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
		writeZstdJSON(w, meta)
	}
}

func writeZstdJSON(w http.ResponseWriter, v any) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "zstd")
	_, _ = w.Write(zstdEncoder.EncodeAll(jsonBytes, nil))
}
