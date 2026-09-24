package datasource

import (
	"net/http"

	"backend/internal/middlewares"

	"github.com/selectDb/dialect/engine"
)

func DumpHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		workspaceID := middlewares.MemberWorkspaceID(r)

		ds, err := GetOrLoadDatasource(r.Context(), id, workspaceID)
		if err != nil {
			http.Error(w, "datasource not found", http.StatusNotFound)
			return
		}

		if ds.CellarID != "" {
			client, _, err := OnCellar(r, id, workspaceID, ds)
			if err != nil {
				cellarError(w, err, "datasource dump", workspaceID, id)
				return
			}
			schemaSQL, err := client.Transport.DumpSchema(r.Context(), workspaceID, id)
			if err != nil {
				cellarError(w, err, "datasource dump", workspaceID, id)
				return
			}
			writeZstdJSON(w, map[string]string{"sql": schemaSQL})
			return
		}

		dbConn, err := engine.GetOrOpenConn(workspaceID, ds.DBType, ds.DSN, ds.SSH, ds.Pool)
		if err != nil {
			http.Error(w, safeConnErr(err, "datasource dump", workspaceID, id), http.StatusBadGateway)
			return
		}

		dialect := engine.GetDialect(ds.DBType)
		if dialect == nil {
			http.Error(w, "unsupported database type", http.StatusBadRequest)
			return
		}

		meta, err := engine.GetOrFetchMetadata(r.Context(), workspaceID, ds.DSN, dbConn, dialect, "", false)
		if err != nil {
			http.Error(w, safeConnErr(err, "datasource dump", workspaceID, id), http.StatusBadGateway)
			return
		}

		// CLI tools dial the host themselves (no Go guard); pin to the same
		// validated endpoint as the driver
		dumpDSN, err := engine.ResolveDumpDSN(workspaceID, ds.DBType, ds.DSN, ds.SSH)
		if err != nil {
			http.Error(w, safeConnErr(err, "datasource dump", workspaceID, id), http.StatusBadGateway)
			return
		}

		schemaSQL := engine.GetOrGenerateDump(dialect, workspaceID, dumpDSN, meta, false)

		writeZstdJSON(w, map[string]string{"sql": schemaSQL})
	}
}
