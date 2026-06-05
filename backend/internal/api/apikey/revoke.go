package apikey

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
)

type revokeRequest struct {
	ID string `json:"id"`
}

func RevokeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := guard(w, r)
		if !ok {
			return
		}

		var req revokeRequest
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

		if err := db.Queries.RevokeAPIKey(r.Context(), idUUID); err != nil {
			http.Error(w, "failed to revoke api key", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
