package managed

import (
	"context"

	"backend/db"
	"backend/db/generated"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/google/uuid"
)

// stateDeleting marks a managed database that stopped serving and waits for
// the reconciler to purge its file.
const stateDeleting = "deleting"

// CheckAvailable refuses a managed database while managed databases are off
// (CELLAR unset), or once it is being deleted.
func CheckAvailable(row generated.GetDatasourceRow) error {
	if cellarclient.URL == "" {
		return cellarclient.ErrOff
	}
	if row.State.ValueOrEmpty() == stateDeleting {
		return ErrNotFound
	}
	return nil
}

// getRow is a managed database the workspace still serves; anything else is
// ErrNotFound.
func getRow(ctx context.Context, id, workspaceID string) (generated.GetDatasourceRow, error) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return generated.GetDatasourceRow{}, ErrNotFound
	}
	row, err := db.Queries.GetDatasource(ctx, generated.GetDatasourceParams{ID: parsedID, WorkspaceID: uuid.MustParse(workspaceID)})
	if err != nil || row.CellarID.ValueOrEmpty() == "" {
		return generated.GetDatasourceRow{}, ErrNotFound
	}
	return row, CheckAvailable(row)
}

// DSN is how the backend reaches managed database row on its cellar, capped
// by the workspace's plan.
func DSN(ctx context.Context, row generated.GetDatasourceRow, id, workspaceID string) (string, error) {
	if err := CheckAvailable(row); err != nil {
		return "", err
	}
	workspace, err := db.Queries.GetWorkspacePlan(ctx, uuid.MustParse(workspaceID))
	if err != nil {
		return "", err
	}
	return cellarclient.DSN(row.CellarID.ValueOrEmpty(), id, workspaceID, plans[workspace.Plan].DatabaseMaxBytes, int(workspace.Members)), nil
}
