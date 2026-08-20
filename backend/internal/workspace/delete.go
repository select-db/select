package workspace

import (
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/audit"
	"backend/internal/middlewares"
)

func DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, ok := middlewares.MustGetUserID(w, r)
		if !ok {
			return
		}

		workspaceID := r.PathValue("id")
		if workspaceID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		userUUID, err := db_types.NewJSONNullUUIDFromString(userID)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusInternalServerError)
			return
		}

		workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace id", http.StatusBadRequest)
			return
		}

		ownerID, err := db.Queries.GetWorkspaceOwnerID(r.Context(), workspaceUUID)
		if err != nil {
			http.Error(w, "workspace not found", http.StatusNotFound)
			return
		}

		if ownerID.String() != userID {
			audit.EmitDenied(r.Context(), audit.WorkspaceDeleted, workspaceID, workspaceID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if err := db.Queries.SetWorkspaceDeletedAt(r.Context(), workspaceUUID); err != nil {
			http.Error(w, "failed to delete workspace", http.StatusInternalServerError)
			return
		}

		_ = db.Queries.DeleteUserRefreshTokens(r.Context(), userUUID)

		audit.EmitAction(r.Context(), audit.WorkspaceDeleted, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    workspaceID,
			Status:      audit.StatusSuccess,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}
