package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/middlewares"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"

	"github.com/google/uuid"
)

type deleteRequest struct {
	ID string `json:"id"`
}

func DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req deleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		workspaceID := middlewares.MemberWorkspaceID(r)

		if !authz.IsWorkspaceOwner(r, workspaceID) && !authz.CompiledFromRequest(r).CanManage(req.ID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		id, err := uuid.Parse(req.ID)
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

		InvalidateCache(workspaceID, req.ID)

		audit.EmitAction(r.Context(), audit.DatasourceDeleted, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    req.ID,
			Status:      audit.StatusSuccess,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
