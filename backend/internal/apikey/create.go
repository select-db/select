package apikey

import (
	"encoding/json"
	"net/http"
	"strings"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/auth"
	"backend/internal/syncer/scope"
)

type createRequest struct {
	Name      string   `json:"name"`
	RoleIDs   []string `json:"role_ids"`
	ExpiresAt *string  `json:"expires_at"`
}

type createResponse struct {
	ID     string `json:"id"`
	Prefix string `json:"prefix"`
	Key    string `json:"key"` // plaintext, returned once and never again
}

func CreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, userID, ok := guard(w, r)
		if !ok {
			return
		}

		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if len(req.RoleIDs) == 0 {
			http.Error(w, "at least one role is required", http.StatusBadRequest)
			return
		}
		expiry, err := resolveExpiry(req.ExpiresAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		wsUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace id", http.StatusInternalServerError)
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

		plaintext, prefix, hash := auth.GenerateAPIKey()

		createdBy, _ := db_types.NewJSONNullUUIDFromString(userID)
		expiresAt := db_types.JSONNullTime{}
		if expiry != nil {
			expiresAt = db_types.NewJSONNullTimeFromTime(*expiry)
		}

		tx, err := db.GetDB().BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "failed to create api key", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()
		q := db.Queries.WithTx(tx)

		created, err := q.CreateAPIKey(r.Context(), generated.CreateAPIKeyParams{
			WorkspaceID: wsUUID,
			Name:        db_types.NewJSONNullString(name),
			Prefix:      db_types.NewJSONNullString(prefix),
			HashedKey:   db_types.NewJSONNullString(hash),
			CreatedBy:   createdBy,
			ExpiresAt:   expiresAt,
		})
		if err != nil {
			http.Error(w, "failed to create api key", http.StatusInternalServerError)
			return
		}
		for _, ru := range roleUUIDs {
			if err := q.AddAPIKeyRole(r.Context(), generated.AddAPIKeyRoleParams{
				ApiKeyID: created.ID,
				RoleID:   ru,
			}); err != nil {
				http.Error(w, "failed to bind role", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to create api key", http.StatusInternalServerError)
			return
		}

		writeJSON(w, createResponse{
			ID:     created.ID.String(),
			Prefix: created.Prefix.ValueOrEmpty(),
			Key:    plaintext,
		})
	}
}
