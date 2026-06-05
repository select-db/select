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
	"backend/internal/authz"
	"backend/internal/middlewares"

	core "github.com/selectDb/dialect/core"
)

type searchUserRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Email       string `json:"email"`
}

type searchUserResponse struct {
	Found        bool    `json:"found"`
	UserID       string  `json:"user_id,omitempty"`
	Name         *string `json:"name,omitempty"`
	Email        string  `json:"email"`
	AlreadyAdded bool    `json:"already_added"`
}

func SearchUserHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req searchUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" {
			http.Error(w, "email is required", http.StatusBadRequest)
			return
		}

		workspaceID := middlewares.MemberWorkspaceID(r)

		if !authz.IsWorkspaceOwner(r, workspaceID) {
			compiled := authz.CompiledFromRequest(r)
			if !compiled.IsAllowed(core.ActionWorkspaceUsersManage) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace id", http.StatusInternalServerError)
			return
		}

		row, err := db.Queries.GetUserByEmail(r.Context(), generated.GetUserByEmailParams{
			Lower:       email,
			WorkspaceID: workspaceUUID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(searchUserResponse{
				Found: false,
				Email: email,
			})
			return
		}
		if err != nil {
			http.Error(w, "failed to search user", http.StatusInternalServerError)
			return
		}

		resp := searchUserResponse{
			Found:        true,
			UserID:       row.ID.String(),
			Email:        row.Email.String,
			AlreadyAdded: row.IsMember,
		}
		if row.Name.Valid {
			name := row.Name.String
			resp.Name = &name
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
