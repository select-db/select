package workspace

import (
	"encoding/json"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/middlewares"
)

type deleteRequest struct {
	ID string `json:"id"`
}

func DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := middlewares.MustGetUserID(w, r)
		if !ok {
			return
		}

		var req deleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		workspaceID := req.ID
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
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if err := db.Queries.SetWorkspaceDeletedAt(r.Context(), workspaceUUID); err != nil {
			http.Error(w, "failed to delete workspace", http.StatusInternalServerError)
			return
		}

		_ = db.Queries.DeleteUserRefreshTokens(r.Context(), userUUID)

		w.WriteHeader(http.StatusNoContent)
	}
}
