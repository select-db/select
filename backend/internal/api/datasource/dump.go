package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/api/middlewares"

	"github.com/selectDb/dialect/engine"
)

type dumpRequest struct {
	ID string `json:"id"`
}

func DumpHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req dumpRequest
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
			http.Error(w, safeConnErr(err, "datasource dump", workspaceID, req.ID), http.StatusBadGateway)
			return
		}

		dialect := engine.GetDialect(ds.DBType)
		if dialect == nil {
			http.Error(w, "unsupported database type", http.StatusBadRequest)
			return
		}

		meta, err := engine.GetOrFetchMetadata(r.Context(), workspaceID, ds.DSN, dbConn, dialect, "", false)
		if err != nil {
			http.Error(w, safeConnErr(err, "datasource dump", workspaceID, req.ID), http.StatusBadGateway)
			return
		}

		// CLI tools dial the host themselves (no Go guard); pin to the same
		// validated endpoint as the driver
		dumpDSN, err := engine.ResolveDumpDSN(workspaceID, ds.DBType, ds.DSN, ds.SSH)
		if err != nil {
			http.Error(w, safeConnErr(err, "datasource dump", workspaceID, req.ID), http.StatusBadGateway)
			return
		}

		schemaSQL := engine.GetOrGenerateDump(dialect, workspaceID, dumpDSN, meta, false)

		jsonBytes, _ := json.Marshal(map[string]string{"sql": schemaSQL})
		compressed := zstdEncoder.EncodeAll(jsonBytes, nil)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "zstd")
		_, _ = w.Write(compressed)
	}
}
