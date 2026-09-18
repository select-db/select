package workspace

import (
	"encoding/json"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/auth"
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

		userUUID, err := uuid.Parse(userID)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusInternalServerError)
			return
		}

		workspaceID := uuid.New()
		wtuID := uuid.New()

		if err := db.Queries.UpsertWorkspace(r.Context(), generated.UpsertWorkspaceParams{
			ID:      workspaceID,
			Name:    req.Name,
			OwnerID: db_types.NewJSONNullUUID(userUUID),
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

		audit.EmitAction(r.Context(), audit.WorkspaceCreated, audit.Record{
			WorkspaceID: workspaceID.String(),
			TargetID:    workspaceID.String(),
			TargetLabel: req.Name,
			Status:      audit.StatusSuccess,
		})

		// The caller's token predates the workspace, so it carries neither the
		// ownership nor the roles they now hold in it, and every owner-gated route
		// there would refuse them until it expired. Hand back one that knows, the
		// way the syncer does for a commit that shifts the caller's own claims.
		if token, err := auth.CreateJWT(r.Context(), userUUID); err == nil {
			w.Header().Set("X-New-Access-Token", token)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(createResponse{
			ID:                workspaceID.String(),
			WorkspaceToUserID: wtuID.String(),
			Name:              req.Name,
			OwnerID:           userID,
		})
	}
}
