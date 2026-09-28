package datasource

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"
	"backend/internal/cellar"
	managed "backend/internal/datasource/cellar"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine/arrowstream"
)

// grantTo names who gets a new managed database's dedicated role. Empty means
// nobody, as for any datasource.
type grantTo struct {
	Users   []string `json:"users"`
	APIKeys []string `json:"api_keys"`
}

type createRequest struct {
	upsertRequest
	GrantTo grantTo `json:"grant_to"`
}

type forkRequest struct {
	Name    string  `json:"name"`
	At      string  `json:"at"`
	GrantTo grantTo `json:"grant_to"`
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

// fullAccess is every action the dedicated role of a managed database allows.
var fullAccess = []string{core.ActionSee, core.ActionSelect, core.ActionInsert, core.ActionUpdate, core.ActionDelete, core.ActionManage}

// requestError is a refusal the caller can act on, answered with its status.
type requestError struct {
	status int
	msg    string
}

func (e *requestError) Error() string { return e.msg }

var errForbidden = &requestError{http.StatusForbidden, "forbidden"}

// CreateHandler adds a datasource under an id the server picks. A SQLite
// datasource without a DSN is a managed database, made on the cellar.
func CreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.ID = uuid.NewString()
		if req.DBType != "sqlite" || req.DSN != "" {
			if store(w, r, req.upsertRequest) {
				writeCreated(w, req.ID, req.DBType)
			}
			return
		}
		a := authz.ActorOf(r)
		if !a.IsOwner() && !a.Can(core.ActionManage) {
			audit.EmitDenied(r.Context(), audit.DatasourceCreated, a.WorkspaceID, req.ID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := createManaged(r, a, req.ID, req.Name, "", "", req.GrantTo); err != nil {
			OpenError(w, err, "managed create", a.WorkspaceID, req.ID)
			return
		}
		writeCreated(w, req.ID, "sqlite")
	}
}

// ForkHandler copies a managed database into a new one; the source is never
// touched. Forking hands over all the data, so it needs manage on the source.
func ForkHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req forkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		a := authz.ActorOf(r)
		source := r.PathValue("id")
		id := uuid.NewString()
		if !a.IsOwner() && !a.CanManage(source) {
			audit.EmitDenied(r.Context(), audit.DatasourceCreated, a.WorkspaceID, id)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		row, err := managedRow(r.Context(), source, a.WorkspaceID)
		if err != nil {
			OpenError(w, err, "managed fork", a.WorkspaceID, source)
			return
		}
		if req.Name == "" {
			req.Name = row.Name + " (fork)"
		}
		if err := createManaged(r, a, id, req.Name, source, req.At, req.GrantTo); err != nil {
			OpenError(w, err, "managed fork", a.WorkspaceID, source)
			return
		}
		writeCreated(w, id, "sqlite")
	}
}

// DownloadHandler sends a managed database's file. It hands over all the
// data, so it needs manage.
func DownloadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		id := r.PathValue("id")
		if !a.IsOwner() && !a.CanManage(id) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		row, err := managedRow(r.Context(), id, a.WorkspaceID)
		if err != nil {
			OpenError(w, err, "managed download", a.WorkspaceID, id)
			return
		}
		plan, err := db.Queries.GetWorkspacePlan(r.Context(), uuid.MustParse(a.WorkspaceID))
		if err != nil {
			OpenError(w, err, "managed download", a.WorkspaceID, id)
			return
		}
		file, err := managed.Download(r.Context(), managed.DSN(row.CellarID.ValueOrEmpty(), id, a.WorkspaceID, plan.Plan, int(plan.Members)))
		if err != nil {
			OpenError(w, err, "managed download", a.WorkspaceID, id)
			return
		}
		defer func() { _ = file.Close() }()
		w.Header().Set("Content-Type", "application/vnd.sqlite3")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName(row.Name)+".db"))
		_, _ = io.Copy(w, file)
	}
}

// managedRow is a managed database the workspace still serves. Anything else
// is ErrNotFound, and while CELLAR is unset every managed route is off.
func managedRow(ctx context.Context, id, workspaceID string) (generated.GetDatasourceRow, error) {
	if managed.URL == "" {
		return generated.GetDatasourceRow{}, managed.ErrOff
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return generated.GetDatasourceRow{}, ErrNotFound
	}
	row, err := db.Queries.GetDatasource(ctx, generated.GetDatasourceParams{ID: parsed, WorkspaceID: uuid.MustParse(workspaceID)})
	if err != nil || row.CellarID.ValueOrEmpty() == "" || row.State.ValueOrEmpty() == "deleting" {
		return generated.GetDatasourceRow{}, ErrNotFound
	}
	return row, nil
}

// createManaged makes managed database id, empty or a fork of source at a
// point in time, with its dedicated role. The workspace row stays locked from
// the quota check to the insert, so two creates cannot both take the last slot.
func createManaged(r *http.Request, a authz.Actor, id, name, source, at string, grants grantTo) error {
	ctx := r.Context()
	if managed.URL == "" {
		return managed.ErrOff
	}
	workspaceID := uuid.MustParse(a.WorkspaceID)
	users, keys, err := checkGrants(ctx, a, workspaceID, grants)
	if err != nil {
		return err
	}

	tx, err := db.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := db.Queries.WithTx(tx)

	plan, err := q.LockWorkspacePlan(ctx, workspaceID)
	if err != nil {
		return err
	}
	limits := managed.Plans[plan]
	usage, err := q.GetManagedUsage(ctx, workspaceID)
	if err != nil {
		return err
	}
	var sourceBytes int64
	if source != "" {
		row, err := q.GetDatasource(ctx, generated.GetDatasourceParams{ID: uuid.MustParse(source), WorkspaceID: workspaceID})
		if err != nil {
			return err
		}
		sourceBytes = row.SizeBytes.Int64
	}
	if err := checkAt(at, limits.PITRDays); err != nil {
		return err
	}
	switch {
	case usage.Dbs >= limits.MaxDBs:
		return &arrowstream.Error{Code: cellar.CodeQuotaExceeded, Message: fmt.Sprintf("the workspace already has its %d managed databases", limits.MaxDBs)}
	case usage.TotalBytes+sourceBytes > limits.TotalBytes:
		return &arrowstream.Error{Code: cellar.CodeQuotaExceeded, Message: fmt.Sprintf("the workspace's managed databases would exceed %d MB", limits.TotalBytes>>20)}
	}

	size, err := managed.Create(ctx, managed.DSN(managed.ID, id, a.WorkspaceID, plan, 0), source, at)
	if err != nil {
		return err
	}
	if err := q.InsertManagedDatasource(ctx, generated.InsertManagedDatasourceParams{
		ID:          uuid.MustParse(id),
		WorkspaceID: workspaceID,
		Name:        name,
		CellarID:    db_types.NewJSONNullString(managed.ID),
		SizeBytes:   db_types.NewJSONNullInt64(size),
	}); err != nil {
		return err
	}
	roleID, err := createRole(ctx, q, workspaceID, id, name, users, keys)
	if err != nil {
		return err
	}
	// A commit that fails here leaves a file with no row, which the reconciler purges.
	if err := tx.Commit(); err != nil {
		return err
	}

	payload := map[string]any{"db_type": "sqlite", "managed": true, "role_id": roleID.String(), "grant_to": grants}
	if source != "" {
		payload["from"] = source
	}
	audit.EmitAction(ctx, audit.DatasourceCreated, audit.Record{
		WorkspaceID: a.WorkspaceID,
		TargetID:    id,
		TargetLabel: name,
		Status:      audit.StatusSuccess,
		Payload:     payload,
	})
	return nil
}

// checkGrants parses who gets the new role. The caller may always grant itself;
// granting anyone else takes the right to manage users or API keys.
func checkGrants(ctx context.Context, a authz.Actor, workspaceID uuid.UUID, grants grantTo) ([]uuid.UUID, []uuid.UUID, error) {
	var users, keys []uuid.UUID
	for _, s := range grants.Users {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, nil, &requestError{http.StatusBadRequest, "invalid user id in grant_to"}
		}
		self := !a.IsAPIKey && s == a.UserID
		if !self && !a.IsOwner() && !a.Can(core.ActionWorkspaceUsersManage) {
			return nil, nil, errForbidden
		}
		member, err := db.Queries.IsWorkspaceMember(ctx, generated.IsWorkspaceMemberParams{WorkspaceID: workspaceID, UserID: id})
		if err != nil {
			return nil, nil, err
		}
		if !member {
			return nil, nil, &requestError{http.StatusBadRequest, "grant_to names a user who is not a member of this workspace"}
		}
		users = append(users, id)
	}
	for _, s := range grants.APIKeys {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, nil, &requestError{http.StatusBadRequest, "invalid api key id in grant_to"}
		}
		self := a.IsAPIKey && s == a.UserID
		if !self && !a.IsOwner() && !a.Can(core.ActionWorkspaceApiKeysManage) {
			return nil, nil, errForbidden
		}
		if _, err := db.Queries.GetAPIKeyForWorkspace(ctx, generated.GetAPIKeyForWorkspaceParams{ID: id, WorkspaceID: workspaceID}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, &requestError{http.StatusBadRequest, "grant_to names an api key of another workspace"}
			}
			return nil, nil, err
		}
		keys = append(keys, id)
	}
	return users, keys, nil
}

// checkAt refuses a point in time outside the plan's window, or in the future.
func checkAt(at string, days int) error {
	if at == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return &requestError{http.StatusBadRequest, "at must be an RFC 3339 time"}
	}
	if t.After(time.Now()) || t.Before(time.Now().AddDate(0, 0, -days)) {
		return &requestError{http.StatusBadRequest, fmt.Sprintf("at must be within the last %d days", days)}
	}
	return nil
}

// createRole gives managed database id a role with full access on it, held by
// users and keys. Rows written here reach clients through sync like any other.
func createRole(ctx context.Context, q *generated.Queries, workspaceID uuid.UUID, id, name string, users, keys []uuid.UUID) (uuid.UUID, error) {
	roleID := uuid.New()
	if err := q.UpsertRole(ctx, generated.UpsertRoleParams{ID: roleID, WorkspaceID: workspaceID, Name: name}); err != nil {
		return roleID, err
	}
	for _, action := range fullAccess {
		if err := q.UpsertPermission(ctx, generated.UpsertPermissionParams{
			ID:           uuid.New(),
			RoleID:       roleID,
			WorkspaceID:  workspaceID,
			DatasourceID: db_types.NewJSONNullString(id),
			Action:       action,
			Effect:       "allow",
		}); err != nil {
			return roleID, err
		}
	}
	for _, user := range users {
		if err := q.UpsertUserToRole(ctx, generated.UpsertUserToRoleParams{ID: uuid.New(), UserID: user, RoleID: roleID, WorkspaceID: workspaceID}); err != nil {
			return roleID, err
		}
	}
	for _, key := range keys {
		if err := q.AddAPIKeyRole(ctx, generated.AddAPIKeyRoleParams{ApiKeyID: key, RoleID: roleID}); err != nil {
			return roleID, err
		}
	}
	return roleID, nil
}

// deleteManaged stops serving a managed database at once and leaves its file
// to the reconciler. Its rules go from every role, and a role left with no
// rule anywhere else goes with them.
func deleteManaged(ctx context.Context, workspaceID uuid.UUID, id string) error {
	tx, err := db.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := db.Queries.WithTx(tx)

	if err := q.MarkDatasourceDeleting(ctx, generated.MarkDatasourceDeletingParams{ID: uuid.MustParse(id), WorkspaceID: workspaceID}); err != nil {
		return err
	}
	scoped, err := q.ListRolesScopedToDatasource(ctx, generated.ListRolesScopedToDatasourceParams{WorkspaceID: workspaceID, DatasourceID: id})
	if err != nil {
		return err
	}
	for _, roleID := range scoped {
		if err := q.SetRoleDeletedAt(ctx, generated.SetRoleDeletedAtParams{ID: roleID, WorkspaceID: workspaceID}); err != nil {
			return err
		}
	}
	touched, err := q.DeleteDatasourcePermissions(ctx, generated.DeleteDatasourcePermissionsParams{WorkspaceID: workspaceID, DatasourceID: id})
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, roleID := range touched {
		authz.Invalidate(roleID.String())
	}
	return nil
}

func writeCreated(w http.ResponseWriter, id, dbType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createResponse{
		ID:     id,
		Config: datasourceConfig{ID: id, DBType: dbType, Proxified: true},
	})
}

// fileName keeps a download's name to characters every file system accepts.
func fileName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	if clean == "" {
		return "database"
	}
	return clean
}
