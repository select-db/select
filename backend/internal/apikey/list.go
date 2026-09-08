package apikey

import (
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/authz"

	"github.com/google/uuid"

	"time"
)

type roleRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// The NOT NULL columns are plain Go values now. The wire shape is unchanged:
// a wrapper around a column that could never be NULL always marshalled the
// value itself. last_used_at and expires_at really are nullable and keep it.
type keyEntry struct {
	ID         uuid.UUID             `json:"id"`
	Name       string                `json:"name"`
	Prefix     string                `json:"prefix"`
	Roles      []roleRef             `json:"roles"`
	CreatedBy  db_types.JSONNullUUID `json:"created_by"`
	CreatedAt  time.Time             `json:"created_at"`
	LastUsedAt db_types.JSONNullTime `json:"last_used_at"`
	ExpiresAt  db_types.JSONNullTime `json:"expires_at"`
}

func ListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		if a.IsAPIKey {
			http.Error(w, "api keys cannot manage api keys", http.StatusForbidden)
			return
		}
		if !a.IsOwner() && !a.Can(manageAPIKeys) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		workspaceID := a.WorkspaceID
		wsUUID, err := uuid.Parse(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace id", http.StatusInternalServerError)
			return
		}

		keys, err := db.Queries.ListAPIKeysByWorkspace(r.Context(), wsUUID)
		if err != nil {
			http.Error(w, "failed to list api keys", http.StatusInternalServerError)
			return
		}
		roleRows, err := db.Queries.ListAPIKeyRolesByWorkspace(r.Context(), wsUUID)
		if err != nil {
			http.Error(w, "failed to list api key roles", http.StatusInternalServerError)
			return
		}

		rolesByKey := make(map[string][]roleRef, len(keys))
		for _, rr := range roleRows {
			id := rr.ApiKeyID.String()
			rolesByKey[id] = append(rolesByKey[id], roleRef{
				ID:   rr.RoleID.String(),
				Name: rr.RoleName,
			})
		}

		out := make([]keyEntry, 0, len(keys))
		for _, k := range keys {
			roles := rolesByKey[k.ID.String()]
			if roles == nil {
				roles = []roleRef{}
			}
			out = append(out, keyEntry{
				ID:         k.ID,
				Name:       k.Name,
				Prefix:     k.Prefix,
				Roles:      roles,
				CreatedBy:  k.CreatedBy,
				CreatedAt:  k.CreatedAt,
				LastUsedAt: k.LastUsedAt,
				ExpiresAt:  k.ExpiresAt,
			})
		}
		writeJSON(w, out)
	}
}
