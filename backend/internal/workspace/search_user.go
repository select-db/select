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

	core "github.com/selectDb/dialect/core"
)

type searchUserResponse struct {
	Found        bool    `json:"found"`
	UserID       string  `json:"user_id,omitempty"`
	Name         *string `json:"name,omitempty"`
	Email        string  `json:"email"`
	AlreadyAdded bool    `json:"already_added"`
}

func SearchUserHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		email := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("email")))
		if email == "" {
			http.Error(w, "email is required", http.StatusBadRequest)
			return
		}

		a := authz.ActorOf(r)
		workspaceID := a.WorkspaceID

		if !a.IsOwner() && !a.Can(core.ActionWorkspaceUsersManage) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
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
