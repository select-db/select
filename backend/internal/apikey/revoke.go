package apikey

import (
	"database/sql"
	"errors"
	"net/http"

	"backend/db"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"

	"github.com/google/uuid"
)

func RevokeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		if a.IsAPIKey {
			audit.EmitDenied(r.Context(), audit.APIKeyRevoked, a.WorkspaceID, "")
			http.Error(w, "api keys cannot manage api keys", http.StatusForbidden)
			return
		}
		if !a.IsOwner() && !a.Can(manageAPIKeys) {
			audit.EmitDenied(r.Context(), audit.APIKeyRevoked, a.WorkspaceID, "")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		workspaceID := a.WorkspaceID

		idUUID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		wsUUID, err := uuid.Parse(workspaceID)
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

		if err := db.Queries.RevokeAPIKey(r.Context(), idUUID); err != nil {
			http.Error(w, "failed to revoke api key", http.StatusInternalServerError)
			return
		}

		audit.EmitAction(r.Context(), audit.APIKeyRevoked, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    idUUID.String(),
			Status:      audit.StatusSuccess,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
