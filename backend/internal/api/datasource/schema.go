package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/api/middlewares"

	"github.com/klauspost/compress/zstd"
	"github.com/selectDb/dialect/engine"
)

type schemaRequest struct {
	ID      string `json:"id"`
	NoCache bool   `json:"no_cache,omitempty"`
}

var zstdEncoder, _ = zstd.NewWriter(nil)

func SchemaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req schemaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		workspaceID := middlewares.MemberWorkspaceID(r)

		ds, err := GetOrLoadDatasource(r.Context(), req.ID, workspaceID)
		if err != nil {
			http.Error(w, "datasource not found", http.StatusNotFound)
			return
		}

		dbConn, err := engine.GetOrOpenConn(workspaceID, ds.DBType, ds.DSN, ds.SSH, ds.Pool)
		if err != nil {
			http.Error(w, safeConnErr(err, "datasource schema", workspaceID, req.ID), http.StatusBadGateway)
			return
		}

		dialect := engine.GetDialect(ds.DBType)
		if dialect == nil {
			http.Error(w, "unsupported database type", http.StatusBadRequest)
			return
		}

		meta, err := engine.GetOrFetchMetadata(r.Context(), workspaceID, ds.DSN, dbConn, dialect, "", req.NoCache)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonBytes, err := json.Marshal(meta)
		if err != nil {
			http.Error(w, "failed to encode metadata", http.StatusInternalServerError)
			return
		}

		compressed := zstdEncoder.EncodeAll(jsonBytes, nil)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "zstd")
		_, _ = w.Write(compressed)
	}
}
