package datasource

import (
	"net/http"

	"backend/db"
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

		existing, err := db.Queries.GetDatasource(r.Context(), generated.GetDatasourceParams{ID: id, WorkspaceID: parsedWorkspaceID})
		managedDB := err == nil && existing.CellarID.ValueOrEmpty() != ""
		if managedDB {
			if err := servable(existing); err != nil {
				OpenError(w, err, "managed delete", workspaceID, idStr)
				return
			}
			err = deleteManaged(r.Context(), parsedWorkspaceID, idStr)
		} else {
			err = db.Queries.DeleteDatasource(r.Context(), generated.DeleteDatasourceParams{
				ID:          id,
				WorkspaceID: parsedWorkspaceID,
			})
		}
		if err != nil {
			http.Error(w, "failed to delete datasource", http.StatusInternalServerError)
			return
		}

		InvalidateCache(workspaceID, idStr)

		audit.EmitAction(r.Context(), audit.DatasourceDeleted, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    idStr,
			Status:      audit.StatusSuccess,
			Payload:     map[string]any{"managed": managedDB},
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
