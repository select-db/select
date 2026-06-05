package syncer

import (
	"backend/db/db_types"
	"backend/internal/api/auth"
	"backend/internal/api/middlewares"
	"backend/internal/api/syncer/types"
	"encoding/json"
	"net/http"
)

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middlewares.MustGetUserID(w, r)
		if !ok {
			return
		}
		workspaceIDs, ok := middlewares.MustGetWorkspaceIDs(w, r)
		if !ok {
			return
		}
		roleIDs := middlewares.GetRoleIDs(r)
		ownedWorkspaceIDs := middlewares.GetOwnedWorkspaceIDs(r)

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req types.SyncRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		resp, needsTokenRefresh, err := Sync(r.Context(), userID, workspaceIDs, roleIDs, ownedWorkspaceIDs, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if needsTokenRefresh {
			uid, err := db_types.NewJSONNullUUIDFromString(userID)
			if err == nil {
				if token, err := auth.CreateJWT(r.Context(), uid); err == nil {
					w.Header().Set("X-New-Access-Token", token)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}
