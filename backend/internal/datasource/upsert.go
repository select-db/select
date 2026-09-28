package datasource

import (
	"encoding/json"
	"net/http"

	"backend/db"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/core"
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
		if saveDatasource(w, r, req) {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// saveDatasource writes req, or on a managed database only its name, and
// reports whether it did; on false it has answered the request.
func saveDatasource(w http.ResponseWriter, r *http.Request, req upsertRequest) bool {
	actor := authz.ActorOf(r)
	workspaceID := actor.WorkspaceID

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
	existing, err := db.Queries.GetDatasource(r.Context(), generated.GetDatasourceParams{
		ID:          id,
		WorkspaceID: parsedWorkspaceID,
	})
	exists := err == nil
	spec := audit.DatasourceCreated
	if exists {
		spec = audit.DatasourceUpdated
	}

	// Adding a datasource takes manage on "*"; changing one, manage on it.
	allowed := actor.IsOwner() || actor.CanManage(req.ID) || (!exists && actor.Can(core.ActionManage))
	if !allowed {
		audit.EmitDenied(r.Context(), spec, workspaceID, req.ID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}

	if exists && existing.CellarID.ValueOrEmpty() != "" {
		rename := generated.RenameDatasourceParams{ID: id, WorkspaceID: parsedWorkspaceID, Name: req.Name}
		return renameManaged(w, r, existing, rename, spec)
	}

	// Proxified datasources are dialed by this multi-tenant server, so only
	// networked engines are allowed. sqlite (and any local-file driver)
	// would open a path on the server host, not a remote DB.
	if req.DBType != "postgresql" && req.DBType != "mysql" {
		http.Error(w, "unsupported db_type", http.StatusBadRequest)
		return false
	}

	encryptor, err := getWrapper()
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
	if exists {
		existingDSN, dsnErr := decryptField(r.Context(), encryptor, existing.EncryptedDsn, dsnAAD)
		existingSSH, sshErr := decryptField(r.Context(), encryptor, existing.EncryptedSsh, sshAAD)
		if dsnErr == nil && sshErr == nil {
			if existing.DbType == req.DBType || existing.DbType == "" {
				dsnToStore = mergeDSN(req.DBType, req.DSN, existingDSN)
			}
			sshToStore = mergeSSH(req.SSH, existingSSH)
		}
	}

	encryptedDSN, err := encryptField(r.Context(), encryptor, dsnToStore, dsnAAD)
	if err != nil {
		http.Error(w, "failed to store credentials", http.StatusInternalServerError)
		return false
	}
	encryptedSSH, err := encryptField(r.Context(), encryptor, sshToStore, sshAAD)
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

// renameManaged gives a managed database a new name, the only setting it has,
// and reports whether it did; on false it has answered the request.
func renameManaged(w http.ResponseWriter, r *http.Request, existing generated.GetDatasourceRow, rename generated.RenameDatasourceParams, spec audit.Spec) bool {
	workspaceID, id := rename.WorkspaceID.String(), rename.ID.String()
	if err := checkManagedAvailable(existing); err != nil {
		OpenError(w, err, "managed rename", workspaceID, id)
		return false
	}
	if err := db.Queries.RenameDatasource(r.Context(), rename); err != nil {
		http.Error(w, "failed to rename datasource", http.StatusInternalServerError)
		return false
	}
	InvalidateCache(workspaceID, id)
	audit.EmitAction(r.Context(), spec, audit.Record{
		WorkspaceID: workspaceID,
		TargetID:    id,
		TargetLabel: rename.Name,
		Status:      audit.StatusSuccess,
		Payload:     map[string]any{"db_type": existing.DbType, "managed": true},
	})
	return true
}
