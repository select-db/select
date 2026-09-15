package workspace

import (
	"net/http"

	"backend/db"
	"backend/internal/audit"
	"backend/internal/datasource"
	"backend/internal/middlewares"

	"github.com/google/uuid"
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

		userUUID, err := uuid.Parse(userID)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusInternalServerError)
			return
		}

		workspaceUUID, err := uuid.Parse(workspaceID)
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

		// The workspace row is the one place a datasource's standing is
		// written: reads join it, and membership is derived from it, so the
		// deletion above already takes both away. What it does not take away is
		// what is already open -- a decrypted DSN in the cache, a pool, an SSH
		// tunnel -- and those stand for the rest of their TTL unless they are
		// dropped here.
		datasource.InvalidateWorkspaceCache(workspaceID)

		audit.EmitAction(r.Context(), audit.WorkspaceDeleted, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    workspaceID,
			Status:      audit.StatusSuccess,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}
