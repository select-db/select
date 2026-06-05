package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/api/middlewares"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/authz"

	"github.com/google/uuid"
)

type getDatasourceRequest struct {
	ID string `json:"id"`
}

type getDatasourceResponse struct {
	Name            string `json:"name"`
	DSN             string `json:"dsn"`
	SSH             string `json:"ssh"`
	MaxOpenConns    int64  `json:"max_open_conns"`
	MaxIdleConns    int64  `json:"max_idle_conns"`
	ConnMaxLifetime int64  `json:"conn_max_lifetime"`
	ConnMaxIdleTime int64  `json:"conn_max_idle_time"`
}

func GetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req getDatasourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		workspaceID := middlewares.MemberWorkspaceID(r)

		if !authz.IsWorkspaceOwner(r, workspaceID) && !authz.CompiledFromRequest(r).CanManage(req.ID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		key, err := secretKey()
		if err != nil {
			http.Error(w, "server misconfigured", http.StatusInternalServerError)
			return
		}

		parsedID, err := uuid.Parse(req.ID)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		parsedWorkspaceID, err := uuid.Parse(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace_id", http.StatusBadRequest)
			return
		}

		row, err := db.Queries.GetDatasource(r.Context(), generated.GetDatasourceParams{
			ID:          db_types.NewJSONNullUUID(parsedID),
			Key:         key,
			WorkspaceID: db_types.NewJSONNullUUID(parsedWorkspaceID),
		})
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(getDatasourceResponse{
			Name:            row.Name.String,
			DSN:             maskDSN(row.DbType.String, row.Dsn),
			SSH:             maskSSH(row.Ssh),
			MaxOpenConns:    row.MaxOpenConns.Int64,
			MaxIdleConns:    row.MaxIdleConns.Int64,
			ConnMaxLifetime: row.ConnMaxLifetime.Int64,
			ConnMaxIdleTime: row.ConnMaxIdleTime.Int64,
		})
	}
}
