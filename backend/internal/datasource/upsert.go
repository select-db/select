package datasource

import (
	"encoding/json"
	"net/http"

	"backend/db"
	"backend/db/generated"
	"backend/internal/audit"
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
		req.ID = r.PathValue("id")
		if req.ID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		if store(w, r, req) {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// store writes req, or on a managed database only its name, and reports
// whether it did; on false it has answered the request.
func store(w http.ResponseWriter, r *http.Request, req upsertRequest) bool {
	a := authz.ActorOf(r)
	workspaceID := a.WorkspaceID

	id, err := uuid.Parse(req.ID)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return false
	}

	parsedWorkspaceID, err := uuid.Parse(workspaceID)
	if err != nil {
		http.Error(w, "invalid workspace_id", http.StatusBadRequest)
		return false
	}

	// Fetch the current row up front: whether it exists decides create vs.
	// update for the audit event, including a denied attempt, so a block is
	// attributed to the change it would have made, and it feeds the
	// write-only secret merge below.
	existing, existErr := db.Queries.GetDatasource(r.Context(), generated.GetDatasourceParams{
		ID:          id,
		WorkspaceID: parsedWorkspaceID,
	})
	spec := audit.DatasourceCreated
	if existErr == nil {
		spec = audit.DatasourceUpdated
	}

	if !a.IsOwner() && !a.CanManage(req.ID) {
		audit.EmitDenied(r.Context(), spec, workspaceID, req.ID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}

	// A managed database has no connection settings: its name is all there is to change.
	if existErr == nil && existing.CellarID.ValueOrEmpty() != "" {
		if err := servable(existing); err != nil {
			OpenError(w, err, "managed rename", workspaceID, req.ID)
			return false
		}
		if err := db.Queries.RenameDatasource(r.Context(), generated.RenameDatasourceParams{ID: id, WorkspaceID: parsedWorkspaceID, Name: req.Name}); err != nil {
			http.Error(w, "failed to rename datasource", http.StatusInternalServerError)
			return false
		}
		InvalidateCache(workspaceID, req.ID)
		audit.EmitAction(r.Context(), spec, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    req.ID,
			TargetLabel: req.Name,
			Status:      audit.StatusSuccess,
			Payload:     map[string]any{"db_type": existing.DbType, "managed": true},
		})
		return true
	}

	// Proxified datasources are dialed by this multi-tenant server, so only
	// networked engines are allowed. sqlite (and any local-file driver)
	// would open a path on the server host, not a remote DB.
	if req.DBType != "postgresql" && req.DBType != "mysql" {
		http.Error(w, "unsupported db_type", http.StatusBadRequest)
		return false
	}

	enc, err := getWrapper()
	if err != nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return false
	}

	dsnAAD := fieldAAD(parsedWorkspaceID, id, "dsn")
	sshAAD := fieldAAD(parsedWorkspaceID, id, "ssh")

	// Secrets are write-only: a blank incoming password/key means "keep the
	// stored one". Merge against the existing (decrypted) record so the
	// client's auto-save (which sends back the redacted DSN/SSH) cannot
	// clobber a stored credential.
	dsnToStore := req.DSN
	sshToStore := req.SSH
	if existErr == nil {
		existingDSN, derr := decryptField(r.Context(), enc, existing.EncryptedDsn, dsnAAD)
		existingSSH, serr := decryptField(r.Context(), enc, existing.EncryptedSsh, sshAAD)
		if derr == nil && serr == nil {
			if existing.DbType == req.DBType || existing.DbType == "" {
				dsnToStore = mergeDSN(req.DBType, req.DSN, existingDSN)
			}
			sshToStore = mergeSSH(req.SSH, existingSSH)
		}
	}

	encryptedDSN, err := encryptField(r.Context(), enc, dsnToStore, dsnAAD)
	if err != nil {
		http.Error(w, "failed to store credentials", http.StatusInternalServerError)
		return false
	}
	encryptedSSH, err := encryptField(r.Context(), enc, sshToStore, sshAAD)
	if err != nil {
		http.Error(w, "failed to store credentials", http.StatusInternalServerError)
		return false
	}

	if err := db.Queries.UpsertDatasource(r.Context(), generated.UpsertDatasourceParams{
		ID:              id,
		WorkspaceID:     parsedWorkspaceID,
		DbType:          req.DBType,
		Name:            req.Name,
		EncryptedDsn:    encryptedDSN,
		EncryptedSsh:    encryptedSSH,
		MaxOpenConns:    int32(req.MaxOpenConns),
		MaxIdleConns:    int32(req.MaxIdleConns),
		ConnMaxLifetime: int32(req.ConnMaxLifetime),
		ConnMaxIdleTime: int32(req.ConnMaxIdleTime),
	}); err != nil {
		http.Error(w, "failed to store credentials", http.StatusInternalServerError)
		return false
	}

	InvalidateCache(workspaceID, req.ID)

	// Secrets (dsn, ssh) are never logged, only the non-sensitive shape.
	audit.EmitAction(r.Context(), spec, audit.Record{
		WorkspaceID: workspaceID,
		TargetID:    req.ID,
		TargetLabel: req.Name,
		Status:      audit.StatusSuccess,
		Payload:     map[string]any{"db_type": req.DBType},
	})
	return true
}
