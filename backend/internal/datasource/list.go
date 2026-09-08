package datasource

import (
	"encoding/json"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/authz"

	"github.com/google/uuid"
)

type listedDatasource struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	DBType string `json:"db_type"`

	// Whether this actor administrates it. The connections screen offers
	// revoking only where it would be allowed, and the answer is computed here
	// rather than in the client so both come from the same rule.
	CanManage bool `json:"can_manage"`
}

// ListHandler answers what proxified connections this workspace has.
//
// It exists because a connection can outlive every trace of itself in the
// workspace files. The directory naming one is replicated through git, so it
// can be deleted on another machine, in a branch, or outside the app entirely,
// while the credential stays here. Without a list there is no way to see such a
// connection, let alone revoke it: the id needed to name it lived in the file
// that was deleted.
//
// No secrets, and no DSNs. Administrating a connection does not require being
// handed the credential behind it.
func ListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)

		parsedWorkspaceID, err := uuid.Parse(a.WorkspaceID)
		if err != nil {
			http.Error(w, "invalid workspace_id", http.StatusBadRequest)
			return
		}

		rows, err := db.Queries.ListDatasourcesByWorkspace(r.Context(), db_types.NewJSONNullUUID(parsedWorkspaceID))
		if err != nil {
			http.Error(w, "failed to list datasources", http.StatusInternalServerError)
			return
		}

		owner := a.IsOwner()
		out := make([]listedDatasource, 0, len(rows))
		for _, row := range rows {
			id := row.ID.UUID.String()
			out = append(out, listedDatasource{
				ID:        id,
				Name:      row.Name.String,
				DBType:    row.DbType.String,
				CanManage: owner || a.CanManage(id),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}
