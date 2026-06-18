package apikey

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/syncer/scope"
)

type setRolesRequest struct {
	ID      string   `json:"id"`
	RoleIDs []string `json:"role_ids"`
}

// SetRolesHandler replaces a key's bound roles. Parity with the user role
// editor: roles are editable in place, no rotation needed. Zero roles is
// allowed (the key then has no access) to match the user toggle UX.
func SetRolesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := guard(w, r)
		if !ok {
			return
		}

		var req setRolesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		idUUID, err := db_types.NewJSONNullUUIDFromString(req.ID)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		wsUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace id", http.StatusInternalServerError)
			return
		}

		// Scope the key to the caller's workspace before mutating it.
		if _, err := db.Queries.GetAPIKeyForWorkspace(r.Context(), generated.GetAPIKeyForWorkspaceParams{
			ID:          idUUID,
			WorkspaceID: wsUUID,
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "api key not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to look up api key", http.StatusInternalServerError)
			return
		}

		roleUUIDs := make([]db_types.JSONNullUUID, 0, len(req.RoleIDs))
		for _, rid := range req.RoleIDs {
			ru, err := db_types.NewJSONNullUUIDFromString(rid)
			if err != nil {
				http.Error(w, "invalid role id", http.StatusBadRequest)
				return
			}
			inWS, err := scope.RoleInWorkspace(r.Context(), ru, wsUUID)
			if err != nil {
				http.Error(w, "failed to validate role", http.StatusInternalServerError)
				return
			}
			if !inWS {
				http.Error(w, "role does not belong to this workspace", http.StatusBadRequest)
				return
			}
			roleUUIDs = append(roleUUIDs, ru)
		}

		tx, err := db.GetDB().BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "failed to set roles", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()
		q := db.Queries.WithTx(tx)

		if err := q.DeleteAPIKeyRoles(r.Context(), idUUID); err != nil {
			http.Error(w, "failed to set roles", http.StatusInternalServerError)
			return
		}
		for _, ru := range roleUUIDs {
			if err := q.AddAPIKeyRole(r.Context(), generated.AddAPIKeyRoleParams{
				ApiKeyID: idUUID,
				RoleID:   ru,
			}); err != nil {
				http.Error(w, "failed to set roles", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to set roles", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
