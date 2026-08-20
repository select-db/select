package datasource

import (
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"

	"github.com/google/uuid"
)

func DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		if idStr == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		a := authz.ActorOf(r)
		workspaceID := a.WorkspaceID

		if !a.IsOwner() && !a.CanManage(idStr) {
			audit.EmitDenied(r.Context(), audit.DatasourceDeleted, workspaceID, idStr)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		parsedWorkspaceID, err := uuid.Parse(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace_id", http.StatusBadRequest)
			return
		}

		if err := db.Queries.DeleteDatasource(r.Context(), generated.DeleteDatasourceParams{
			ID:          db_types.NewJSONNullUUID(id),
			WorkspaceID: db_types.NewJSONNullUUID(parsedWorkspaceID),
		}); err != nil {
			http.Error(w, "failed to delete datasource", http.StatusInternalServerError)
			return
		}

		InvalidateCache(workspaceID, idStr)

		audit.EmitAction(r.Context(), audit.DatasourceDeleted, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    idStr,
			Status:      audit.StatusSuccess,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
