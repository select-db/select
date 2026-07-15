package workspace

import (
	"encoding/json"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/middlewares"

	"github.com/google/uuid"
)

type createRequest struct {
	Name string `json:"name"`
}

type createResponse struct {
	ID                string `json:"id"`
	WorkspaceToUserID string `json:"workspace_to_user_id"`
	Name              string `json:"name"`
	OwnerID           string `json:"owner_id"`
}

func CreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, ok := middlewares.MustGetUserID(w, r)
		if !ok {
			return
		}

		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		userUUID, err := db_types.NewJSONNullUUIDFromString(userID)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusInternalServerError)
			return
		}

		workspaceID := db_types.NewJSONNullUUID(uuid.New())
		wtuID := db_types.NewJSONNullUUID(uuid.New())

		if err := db.Queries.UpsertWorkspace(r.Context(), generated.UpsertWorkspaceParams{
			ID:      workspaceID,
			Name:    db_types.NewJSONNullString(req.Name),
			OwnerID: userUUID,
		}); err != nil {
			http.Error(w, "failed to create workspace", http.StatusInternalServerError)
			return
		}

		if err := db.Queries.InsertWorkspaceToUser(r.Context(), generated.InsertWorkspaceToUserParams{
			ID:          wtuID,
			WorkspaceID: workspaceID,
			UserID:      userUUID,
		}); err != nil {
			http.Error(w, "failed to link user to workspace", http.StatusInternalServerError)
			return
		}

		// Revoke tokens so the next JWT refresh includes the new workspace in claims.
		_ = db.Queries.DeleteUserRefreshTokens(r.Context(), userUUID)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(createResponse{
			ID:                workspaceID.String(),
			WorkspaceToUserID: wtuID.String(),
			Name:              req.Name,
			OwnerID:           userID,
		})
	}
}
