package datasource

import (
	"context"
	"net/http"

	"backend/internal/middlewares"

	"github.com/selectDb/dialect/dialects"
	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/engine/connect"
)

func DumpHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		workspaceID := middlewares.MemberWorkspaceID(r)

		o, err := Open(r, id, workspaceID)
		if err != nil {
			OpenError(w, err, "datasource dump", workspaceID, id)
			return
		}
		schemaSQL, err := localDump(r.Context(), o)
		if err != nil {
			OpenError(w, err, "datasource dump", workspaceID, id)
			return
		}
		writeZstdJSON(w, map[string]string{"sql": schemaSQL})
	}
}

func localDump(ctx context.Context, o Opened) (string, error) {
	meta, err := o.Metadata(ctx, false)
	if err != nil {
		return "", err
	}
	// CLI tools dial the host themselves (no Go guard); pin to the same
	// validated endpoint as the driver
	dumpDSN, err := connect.ResolveDumpDSN(o.WorkspaceID, o.DS.DBType, o.DS.DSN, o.DS.SSH)
	if err != nil {
		return "", err
	}
	return engine.GetOrGenerateDump(dialects.Get(o.DS.DBType), o.WorkspaceID, dumpDSN, meta, false), nil
}
