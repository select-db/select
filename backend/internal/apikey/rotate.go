package apikey

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/authz"
)

// RotateHandler mints a successor carrying the old key's name, roles, and
// expiry, then shortens the old key to a grace window so a client can swap
// without an outage.
func RotateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		if a.IsAPIKey {
			audit.EmitDenied(r.Context(), audit.APIKeyRotated, a.WorkspaceID, "")
			http.Error(w, "api keys cannot manage api keys", http.StatusForbidden)
			return
		}
		if !a.IsOwner() && !a.Can(manageAPIKeys) {
			audit.EmitDenied(r.Context(), audit.APIKeyRotated, a.WorkspaceID, "")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		workspaceID := a.WorkspaceID
		userID := a.UserID

		oldID, err := db_types.NewJSONNullUUIDFromString(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		wsUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace id", http.StatusInternalServerError)
			return
		}

		old, err := db.Queries.GetAPIKeyForWorkspace(r.Context(), generated.GetAPIKeyForWorkspaceParams{
			ID:          oldID,
			WorkspaceID: wsUUID,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "api key not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to look up api key", http.StatusInternalServerError)
			return
		}
		roleIDs, err := db.Queries.GetAPIKeyRoleIDs(r.Context(), oldID)
		if err != nil {
			http.Error(w, "failed to read api key roles", http.StatusInternalServerError)
			return
		}

		plaintext, prefix, hash := auth.GenerateAPIKey()
		createdBy, _ := db_types.NewJSONNullUUIDFromString(userID)

		tx, err := db.GetDB().BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "failed to rotate api key", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()
		q := db.Queries.WithTx(tx)

		created, err := q.CreateAPIKey(r.Context(), generated.CreateAPIKeyParams{
			WorkspaceID: wsUUID,
			Name:        old.Name,
			Prefix:      db_types.NewJSONNullString(prefix),
			HashedKey:   db_types.NewJSONNullString(hash),
			CreatedBy:   createdBy,
			ExpiresAt:   old.ExpiresAt,
		})
		if err != nil {
			http.Error(w, "failed to rotate api key", http.StatusInternalServerError)
			return
		}
		for _, rid := range roleIDs {
			if err := q.AddAPIKeyRole(r.Context(), generated.AddAPIKeyRoleParams{
				ApiKeyID: created.ID,
				RoleID:   rid,
			}); err != nil {
				http.Error(w, "failed to rotate api key", http.StatusInternalServerError)
				return
			}
		}
		// Mark the old key so the user can tell them apart
		oldName := old.Name.ValueOrEmpty()
		if err := q.RenameAPIKey(r.Context(), generated.RenameAPIKeyParams{
			ID:   oldID,
			Name: db_types.NewJSONNullString(oldName + " (rotated)"),
		}); err != nil {
			http.Error(w, "failed to rotate api key", http.StatusInternalServerError)
			return
		}
		// Shorten (never extend) the old key to the grace window
		if err := q.ExpireAPIKeyAt(r.Context(), generated.ExpireAPIKeyAtParams{
			ID:        oldID,
			ExpiresAt: db_types.NewJSONNullTimeFromTime(time.Now().Add(rotateGraceWindow)),
		}); err != nil {
			http.Error(w, "failed to rotate api key", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to rotate api key", http.StatusInternalServerError)
			return
		}

		audit.EmitAction(r.Context(), audit.APIKeyRotated, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    oldID.String(),
			TargetLabel: old.Name.ValueOrEmpty(),
			Status:      audit.StatusSuccess,
			Payload:     map[string]any{"new_api_key_id": created.ID.String()},
		})

		writeJSON(w, createResponse{
			ID:     created.ID.String(),
			Prefix: created.Prefix.ValueOrEmpty(),
			Key:    plaintext,
		})
	}
}
