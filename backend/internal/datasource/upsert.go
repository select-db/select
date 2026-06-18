package datasource

import (
	"encoding/json"
	"net/http"

	"backend/internal/middlewares"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/authz"

	"github.com/google/uuid"
)

type upsertRequest struct {
	ID              string `json:"id"`
	DBType          string `json:"db_type"`
	Name            string `json:"name"`
	DSN             string `json:"dsn"`
	SSH             string `json:"ssh"`
	MaxOpenConns    int64  `json:"max_open_conns"`
	MaxIdleConns    int64  `json:"max_idle_conns"`
	ConnMaxLifetime int64  `json:"conn_max_lifetime"`
	ConnMaxIdleTime int64  `json:"conn_max_idle_time"`
}

func UpsertHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req upsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		// Proxified datasources are dialed by this multi-tenant server, so only
		// networked engines are allowed. sqlite (and any local-file driver)
		// would open a path on the server host, not a remote DB.
		if req.DBType != "postgresql" && req.DBType != "mysql" {
			http.Error(w, "unsupported db_type", http.StatusBadRequest)
			return
		}

		workspaceID := middlewares.MemberWorkspaceID(r)

		if !authz.IsWorkspaceOwner(r, workspaceID) && !authz.CompiledFromRequest(r).CanManage(req.ID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		enc, err := getWrapper()
		if err != nil {
			http.Error(w, "server misconfigured", http.StatusInternalServerError)
			return
		}

		id, err := uuid.Parse(req.ID)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		parsedWorkspaceID, err := uuid.Parse(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace_id", http.StatusBadRequest)
			return
		}

		dsnAAD := fieldAAD(parsedWorkspaceID, id, "dsn")
		sshAAD := fieldAAD(parsedWorkspaceID, id, "ssh")

		// Secrets are write-only: a blank incoming password/key means "keep the
		// stored one". Merge against the existing (decrypted) record so the
		// client's auto-save (which sends back the redacted DSN/SSH) cannot
		// clobber a stored credential.
		dsnToStore := req.DSN
		sshToStore := req.SSH
		if existing, gerr := db.Queries.GetDatasource(r.Context(), generated.GetDatasourceParams{
			ID:          db_types.NewJSONNullUUID(id),
			WorkspaceID: db_types.NewJSONNullUUID(parsedWorkspaceID),
		}); gerr == nil {
			existingDSN, derr := decryptField(r.Context(), enc, existing.EncryptedDsn, dsnAAD)
			existingSSH, serr := decryptField(r.Context(), enc, existing.EncryptedSsh, sshAAD)
			if derr == nil && serr == nil {
				if existing.DbType.String == req.DBType || existing.DbType.String == "" {
					dsnToStore = mergeDSN(req.DBType, req.DSN, existingDSN)
				}
				sshToStore = mergeSSH(req.SSH, existingSSH)
			}
		}

		encryptedDSN, err := encryptField(r.Context(), enc, dsnToStore, dsnAAD)
		if err != nil {
			http.Error(w, "failed to store credentials", http.StatusInternalServerError)
			return
		}
		encryptedSSH, err := encryptField(r.Context(), enc, sshToStore, sshAAD)
		if err != nil {
			http.Error(w, "failed to store credentials", http.StatusInternalServerError)
			return
		}

		if err := db.Queries.UpsertDatasource(r.Context(), generated.UpsertDatasourceParams{
			ID:              db_types.NewJSONNullUUID(id),
			WorkspaceID:     db_types.NewJSONNullUUID(parsedWorkspaceID),
			DbType:          db_types.NewJSONNullString(req.DBType),
			Name:            db_types.NewJSONNullString(req.Name),
			EncryptedDsn:    encryptedDSN,
			EncryptedSsh:    encryptedSSH,
			MaxOpenConns:    db_types.NewJSONNullInt64(req.MaxOpenConns),
			MaxIdleConns:    db_types.NewJSONNullInt64(req.MaxIdleConns),
			ConnMaxLifetime: db_types.NewJSONNullInt64(req.ConnMaxLifetime),
			ConnMaxIdleTime: db_types.NewJSONNullInt64(req.ConnMaxIdleTime),
		}); err != nil {
			http.Error(w, "failed to store credentials", http.StatusInternalServerError)
			return
		}

		InvalidateCache(workspaceID, req.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}
