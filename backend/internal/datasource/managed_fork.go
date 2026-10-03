package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/audit"
	"backend/internal/authz"
	"backend/internal/datasource/managed"
)

type forkRequest struct {
	Name        string          `json:"name"`
	PointInTime string          `json:"at"`
	GrantTo     managed.GrantTo `json:"grant_to"`
}

// ForkHandler copies a managed database into a new one; the source is never
// touched. Forking hands over all the data, so it needs manage on the source.
func ForkHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, sourceID := authz.ActorOf(r), r.PathValue("id")
		if !actor.IsOwner() && !actor.CanManage(sourceID) {
			audit.EmitDenied(r.Context(), audit.DatasourceCreated, actor.WorkspaceID, sourceID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req forkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		id, err := managed.Create(r.Context(), actor, req.Name, sourceID, req.PointInTime, req.GrantTo)
		if err != nil {
			OpenError(w, err, "managed fork", actor.WorkspaceID, sourceID)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(createResponse{ID: id, Config: datasourceConfig{ID: id, DBType: "sqlite", Proxified: true}})
	}
}
