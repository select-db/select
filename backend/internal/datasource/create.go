package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/audit"
	"backend/internal/authz"
	"backend/internal/datasource/managed"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/core"
)

type createRequest struct {
	upsertRequest
	GrantTo managed.GrantTo `json:"grant_to"`
}

// datasourceConfig is the datasource.config.json that adds a datasource to a
// workspace folder.
type datasourceConfig struct {
	ID        string `json:"id"`
	DBType    string `json:"db_type"`
	DSN       string `json:"dsn"`
	Proxified bool   `json:"proxified"`
}

type createResponse struct {
	ID     string           `json:"id"`
	Config datasourceConfig `json:"config"`
}

// CreateHandler adds a datasource under an id the server picks. A SQLite
// datasource without a DSN is a managed database, made on the cellar.
func CreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		actor, id, dbType := authz.ActorOf(r), uuid.NewString(), req.DBType
		// Adding a datasource, managed or not, takes the workspace permission.
		if !actor.IsOwner() && !actor.Can(core.ActionWorkspaceDatasourcesCreate) {
			audit.EmitDenied(r.Context(), audit.DatasourceCreated, actor.WorkspaceID, id)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if dbType == "sqlite" && req.DSN == "" {
			var err error
			id, err = managed.Create(r.Context(), actor, req.Name, "", "", req.GrantTo)
			if err != nil {
				OpenError(w, err, "managed create", actor.WorkspaceID, id)
				return
			}
		} else {
			req.ID = id
			if !saveDatasource(w, r, req.upsertRequest) {
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(createResponse{ID: id, Config: datasourceConfig{ID: id, DBType: dbType, Proxified: true}})
	}
}
