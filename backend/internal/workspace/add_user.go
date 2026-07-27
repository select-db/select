package workspace

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"

	core "github.com/selectDb/dialect/core"
)

// nameFromEmail extracts a display name from the local part of an email.
// "arthur.mesplede@example.com" → "Arthur Mesplede"
func nameFromEmail(email string) string {
	local := strings.SplitN(email, "@", 2)[0]
	parts := strings.FieldsFunc(local, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == '+'
	})
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

type addUserRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Email       string `json:"email"`
}

type addUserResponse struct {
	UserID            string `json:"user_id"`
	WorkspaceToUserID string `json:"workspace_to_user_id"`
}

func AddUserHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var req addUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" || !strings.Contains(email, "@") {
			http.Error(w, "valid email is required", http.StatusBadRequest)
			return
		}

		a := authz.ActorOf(r)
		workspaceID := a.WorkspaceID

		if !a.IsOwner() && !a.Can(core.ActionWorkspaceUsersManage) {
			audit.EmitDenied(r.Context(), audit.WorkspaceUserAdded, workspaceID, "")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace id", http.StatusInternalServerError)
			return
		}

		// Find existing user by email
		var userUUID db_types.JSONNullUUID
		row, err := db.Queries.GetUserByEmail(r.Context(), generated.GetUserByEmailParams{
			Lower:       email,
			WorkspaceID: workspaceUUID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			// No user with this email: create a placeholder
			userUUID, err = db.Queries.InsertUserPlaceholder(r.Context(), generated.InsertUserPlaceholderParams{
				Email: db_types.NewJSONNullString(email),
				Name:  db_types.NewJSONNullString(nameFromEmail(email)),
			})
			if err != nil {
				http.Error(w, "failed to create user placeholder", http.StatusInternalServerError)
				return
			}
		} else if err != nil {
			http.Error(w, "failed to look up user", http.StatusInternalServerError)
			return
		} else {
			if row.IsMember {
				http.Error(w, "user is already in this workspace", http.StatusConflict)
				return
			}
			userUUID = row.ID
		}

		// Try to reactivate a soft-deleted membership first
		wtuID, err := db.Queries.ReactivateWorkspaceToUser(r.Context(), generated.ReactivateWorkspaceToUserParams{
			WorkspaceID: workspaceUUID,
			UserID:      userUUID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			// No soft-deleted row: insert fresh
			wtuID, err = db.Queries.InsertWorkspaceToUserForAdd(r.Context(), generated.InsertWorkspaceToUserForAddParams{
				WorkspaceID: workspaceUUID,
				UserID:      userUUID,
			})
			if err != nil {
				http.Error(w, "failed to add user", http.StatusInternalServerError)
				return
			}
		} else if err != nil {
			http.Error(w, "failed to reactivate membership", http.StatusInternalServerError)
			return
		}

		audit.EmitAction(r.Context(), audit.WorkspaceUserAdded, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    userUUID.String(),
			Status:      audit.StatusSuccess,
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(addUserResponse{
			UserID:            userUUID.String(),
			WorkspaceToUserID: wtuID.String(),
		})
	}
}
