package datasource

import (
	"context"
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

// managedPlan is what a workspace plan allows its managed databases.
type managedPlan struct {
	DatabaseMaxBytes  int64
	WorkspaceMaxBytes int64 // all its managed databases together
	MaxDatabases      int64
	PointInTimeDays   int // how far back a fork may reach
}

// managedPlans by workspace.plan. An unknown plan has no size cap, which the
// cellar refuses, and room for no database.
var managedPlans = map[string]managedPlan{
	"solo":  {DatabaseMaxBytes: 250 << 20, WorkspaceMaxBytes: 1 << 30, MaxDatabases: 10, PointInTimeDays: 1},
	"teams": {DatabaseMaxBytes: 1 << 30, WorkspaceMaxBytes: 20 << 30, MaxDatabases: 100, PointInTimeDays: 7},
}

// stateDeleting marks a managed database that stopped serving and waits for
// the reconciler to purge its file.
const stateDeleting = "deleting"

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
	return row, checkManagedAvailable(row)
}

// checkManagedAvailable refuses a managed database while managed databases are
// off (CELLAR unset), or once it is being deleted.
func checkManagedAvailable(row generated.GetDatasourceRow) error {
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
	limits := managedPlans[usage.Plan]
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

	dsn := cellarclient.DSN(cellarclient.CellarID, database.ID, actor.WorkspaceID, limits.DatabaseMaxBytes, 0)
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
