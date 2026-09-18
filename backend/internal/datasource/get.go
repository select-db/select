package datasource

import (
	"encoding/json"
	"net/http"

	"backend/db"
	"backend/db/generated"
	"backend/internal/authz"

	"github.com/google/uuid"
)

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
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		a := authz.ActorOf(r)
		workspaceID := a.WorkspaceID

		if !a.IsOwner() && !a.CanManage(id) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		enc, err := getWrapper()
		if err != nil {
			http.Error(w, "server misconfigured", http.StatusInternalServerError)
			return
		}

		parsedID, err := uuid.Parse(id)
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
			ID:          parsedID,
			WorkspaceID: parsedWorkspaceID,
		})
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		dsn, err := decryptField(r.Context(), enc, row.EncryptedDsn, fieldAAD(parsedWorkspaceID, parsedID, "dsn"))
		if err != nil {
			http.Error(w, "failed to read credentials", http.StatusInternalServerError)
			return
		}
		ssh, err := decryptField(r.Context(), enc, row.EncryptedSsh, fieldAAD(parsedWorkspaceID, parsedID, "ssh"))
		if err != nil {
			http.Error(w, "failed to read credentials", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(getDatasourceResponse{
			Name:            row.Name,
			DSN:             maskDSN(row.DbType, dsn),
			SSH:             maskSSH(ssh),
			MaxOpenConns:    int64(row.MaxOpenConns),
			MaxIdleConns:    int64(row.MaxIdleConns),
			ConnMaxLifetime: int64(row.ConnMaxLifetime),
			ConnMaxIdleTime: int64(row.ConnMaxIdleTime),
		})
	}
}
