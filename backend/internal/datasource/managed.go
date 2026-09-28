package datasource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"
	"backend/internal/cellar"
	"backend/internal/datasource/cellarclient"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine/arrowstream"
)

// GrantTo names who gets a new managed database's dedicated role. Empty means
// nobody, as for any datasource.
type GrantTo struct {
	Users   []string `json:"users"`
	APIKeys []string `json:"api_keys"`
}

// stateDeleting marks a managed database that stopped serving and waits for
// the reconciler to purge its file.
const stateDeleting = "deleting"

// fullAccess is every action the dedicated role of a managed database allows.
var fullAccess = []string{core.ActionSee, core.ActionSelect, core.ActionInsert, core.ActionUpdate, core.ActionDelete, core.ActionManage}

// Refusal is a request the caller can correct, answered with its HTTP status.
type Refusal struct {
	Status  int
	Message string
}

func (refusal *Refusal) Error() string { return refusal.Message }

var errForbidden = &Refusal{http.StatusForbidden, "forbidden"}

// getManagedRow is a managed database the workspace still serves; anything
// else is ErrNotFound.
func getManagedRow(ctx context.Context, id, workspaceID string) (generated.GetDatasourceRow, error) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return generated.GetDatasourceRow{}, ErrNotFound
	}
	row, err := db.Queries.GetDatasource(ctx, generated.GetDatasourceParams{ID: parsedID, WorkspaceID: uuid.MustParse(workspaceID)})
	if err != nil || row.CellarID.ValueOrEmpty() == "" {
		return generated.GetDatasourceRow{}, ErrNotFound
	}
	return row, checkServing(row)
}

// checkServing refuses a managed row that no longer serves. While CELLAR is
// unset every managed route is off.
func checkServing(row generated.GetDatasourceRow) error {
	if cellarclient.URL == "" {
		return cellarclient.ErrOff
	}
	if row.State.ValueOrEmpty() == stateDeleting {
		return ErrNotFound
	}
	return nil
}

// newManagedDatabase is a managed database about to be made: empty, or a copy
// of SourceID as it was at PointInTime ("" for now).
type newManagedDatabase struct {
	ID          string
	Name        string
	SourceID    string
	SourceBytes int64
	PointInTime string
	Grants      GrantTo
}

// CreateManaged makes a managed database, empty or a copy of sourceID as it
// was at pointInTime, and returns its id. REST and MCP both create through it.
func CreateManaged(r *http.Request, name, sourceID, pointInTime string, grants GrantTo) (string, error) {
	actor := authz.ActorOf(r)
	database := newManagedDatabase{ID: uuid.NewString(), Name: name, SourceID: sourceID, PointInTime: pointInTime, Grants: grants}
	if cellarclient.URL == "" {
		return database.ID, cellarclient.ErrOff
	}
	// Creating takes the right to add datasources; forking hands over all the
	// source's data, so it takes manage on the source.
	isFork := sourceID != ""
	allowed := actor.IsOwner() || (!isFork && actor.Can(core.ActionManage)) || (isFork && actor.CanManage(sourceID))
	if !allowed {
		audit.EmitDenied(r.Context(), audit.DatasourceCreated, actor.WorkspaceID, database.ID)
		return database.ID, errForbidden
	}
	if isFork {
		source, err := getManagedRow(r.Context(), sourceID, actor.WorkspaceID)
		if err != nil {
			return database.ID, err
		}
		if database.Name == "" {
			database.Name = source.Name + " (fork)"
		}
		database.SourceBytes = source.SizeBytes.Int64
	}
	if database.Name == "" {
		return database.ID, &Refusal{http.StatusBadRequest, "name is required"}
	}
	return database.ID, provisionManaged(r.Context(), actor, database)
}

// provisionManaged checks the quota, has the cellar write the file, then adds
// the datasource row and its dedicated role. The workspace row stays locked
// from the quota check to the insert, so two creates cannot both take the last slot.
func provisionManaged(ctx context.Context, actor authz.Actor, database newManagedDatabase) error {
	workspaceID := uuid.MustParse(actor.WorkspaceID)
	userIDs, apiKeyIDs, err := resolveGrants(ctx, actor, workspaceID, database.Grants)
	if err != nil {
		return err
	}

	tx, err := db.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	queries := db.Queries.WithTx(tx)

	usage, err := queries.LockManagedUsage(ctx, workspaceID)
	if err != nil {
		return err
	}
	plan := usage.Plan
	limits := cellarclient.Plans[plan]
	if err := checkPointInTime(database.PointInTime, limits.PointInTimeDays); err != nil {
		return err
	}
	workspaceFull := &arrowstream.Error{Code: cellar.CodeQuotaExceeded, Message: fmt.Sprintf("the workspace's managed databases would exceed %d MB", limits.WorkspaceMaxBytes>>20)}
	if usage.DatabaseCount >= limits.MaxDatabases {
		return &arrowstream.Error{Code: cellar.CodeQuotaExceeded, Message: fmt.Sprintf("the workspace already has its %d managed databases", limits.MaxDatabases)}
	}
	if usage.TotalBytes+database.SourceBytes > limits.WorkspaceMaxBytes {
		return workspaceFull
	}

	dsn := cellarclient.DSN(cellarclient.CellarID, database.ID, actor.WorkspaceID, plan, 0)
	createdBytes, err := cellarclient.Create(ctx, dsn, database.SourceID, database.PointInTime)
	if err != nil {
		return err
	}
	// The source's recorded size can lag its file; the copy's is exact. A refused
	// copy is an orphan file the reconciler purges.
	if usage.TotalBytes+createdBytes > limits.WorkspaceMaxBytes {
		return workspaceFull
	}
	if err := queries.InsertManagedDatasource(ctx, generated.InsertManagedDatasourceParams{
		ID:          uuid.MustParse(database.ID),
		WorkspaceID: workspaceID,
		Name:        database.Name,
		CellarID:    db_types.NewJSONNullString(cellarclient.CellarID),
		SizeBytes:   db_types.NewJSONNullInt64(createdBytes),
	}); err != nil {
		return err
	}
	roleID, err := createDedicatedRole(ctx, queries, workspaceID, database.ID, database.Name, userIDs, apiKeyIDs)
	if err != nil {
		return err
	}
	// A commit that fails here leaves a file with no row, which the reconciler purges.
	if err := tx.Commit(); err != nil {
		return err
	}

	audit.EmitChange(ctx, audit.RoleCreated, actor.WorkspaceID, roleID.String(), nil, map[string]any{"name": database.Name, "datasource_id": database.ID})
	payload := map[string]any{"db_type": "sqlite", "managed": true, "role_id": roleID.String(), "grant_to": database.Grants}
	if database.SourceID != "" {
		payload["from"] = database.SourceID
	}
	audit.EmitAction(ctx, audit.DatasourceCreated, audit.Record{
		WorkspaceID: actor.WorkspaceID,
		TargetID:    database.ID,
		TargetLabel: database.Name,
		Status:      audit.StatusSuccess,
		Payload:     payload,
	})
	return nil
}

// resolveGrants parses and checks who gets the new role. The caller may always
// grant itself; granting anyone else takes the right to manage users or API keys.
func resolveGrants(ctx context.Context, actor authz.Actor, workspaceID uuid.UUID, grants GrantTo) (userIDs, apiKeyIDs []uuid.UUID, err error) {
	for _, rawID := range grants.Users {
		userID, err := uuid.Parse(rawID)
		if err != nil {
			return nil, nil, &Refusal{http.StatusBadRequest, "invalid user id in grant_to"}
		}
		isCaller := !actor.IsAPIKey && rawID == actor.UserID
		if !isCaller && !actor.IsOwner() && !actor.Can(core.ActionWorkspaceUsersManage) {
			return nil, nil, errForbidden
		}
		isMember, err := db.Queries.IsWorkspaceMember(ctx, generated.IsWorkspaceMemberParams{WorkspaceID: workspaceID, UserID: userID})
		if err != nil {
			return nil, nil, err
		}
		if !isMember {
			return nil, nil, &Refusal{http.StatusBadRequest, "grant_to names a user who is not a member of this workspace"}
		}
		userIDs = append(userIDs, userID)
	}
	for _, rawID := range grants.APIKeys {
		apiKeyID, err := uuid.Parse(rawID)
		if err != nil {
			return nil, nil, &Refusal{http.StatusBadRequest, "invalid api key id in grant_to"}
		}
		// An API key caller's UserID is its key id.
		isCaller := actor.IsAPIKey && rawID == actor.UserID
		if !isCaller && !actor.IsOwner() && !actor.Can(core.ActionWorkspaceApiKeysManage) {
			return nil, nil, errForbidden
		}
		_, err = db.Queries.GetAPIKeyForWorkspace(ctx, generated.GetAPIKeyForWorkspaceParams{ID: apiKeyID, WorkspaceID: workspaceID})
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, &Refusal{http.StatusBadRequest, "grant_to names an api key of another workspace"}
		}
		if err != nil {
			return nil, nil, err
		}
		apiKeyIDs = append(apiKeyIDs, apiKeyID)
	}
	return userIDs, apiKeyIDs, nil
}

// createDedicatedRole gives a managed database a role with full access on it,
// held by userIDs and apiKeyIDs. Its rows reach clients through sync like any other.
func createDedicatedRole(ctx context.Context, queries *generated.Queries, workspaceID uuid.UUID, datasourceID, name string, userIDs, apiKeyIDs []uuid.UUID) (uuid.UUID, error) {
	roleID := uuid.New()
	if err := queries.UpsertRole(ctx, generated.UpsertRoleParams{ID: roleID, WorkspaceID: workspaceID, Name: name}); err != nil {
		return roleID, err
	}
	for _, action := range fullAccess {
		if err := queries.UpsertPermission(ctx, generated.UpsertPermissionParams{
			ID:           uuid.New(),
			RoleID:       roleID,
			WorkspaceID:  workspaceID,
			DatasourceID: db_types.NewJSONNullString(datasourceID),
			Action:       action,
			Effect:       "allow",
		}); err != nil {
			return roleID, err
		}
	}
	for _, userID := range userIDs {
		if err := queries.UpsertUserToRole(ctx, generated.UpsertUserToRoleParams{ID: uuid.New(), UserID: userID, RoleID: roleID, WorkspaceID: workspaceID}); err != nil {
			return roleID, err
		}
	}
	for _, apiKeyID := range apiKeyIDs {
		if err := queries.AddAPIKeyRole(ctx, generated.AddAPIKeyRoleParams{ApiKeyID: apiKeyID, RoleID: roleID}); err != nil {
			return roleID, err
		}
	}
	return roleID, nil
}
