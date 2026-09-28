package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/authz"

	"github.com/google/uuid"
)

type createRequest struct {
	upsertRequest
	GrantTo GrantTo `json:"grant_to"`
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
		if req.DBType != "sqlite" || req.DSN != "" {
			req.ID = uuid.NewString()
			if saveDatasource(w, r, req.upsertRequest) {
				writeCreated(w, req.ID, req.DBType)
			}
			return
		}
		id, err := CreateManaged(r, req.Name, "", "", req.GrantTo)
		if err != nil {
			OpenError(w, err, "managed create", authz.ActorOf(r).WorkspaceID, id)
			return
		}
		writeCreated(w, id, "sqlite")
	}
}

func writeCreated(w http.ResponseWriter, id, dbType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createResponse{
		ID:     id,
		Config: datasourceConfig{ID: id, DBType: dbType, Proxified: true},
	})
}
