package managed

import (
	"context"

	"backend/db"
	"backend/db/generated"
	"backend/internal/audit"
)

// Rename gives a managed database a new name, the only setting it has.
func Rename(ctx context.Context, existing generated.GetDatasourceRow, rename generated.RenameDatasourceParams) error {
	if err := CheckAvailable(existing); err != nil {
		return err
	}
	if err := db.Queries.RenameDatasource(ctx, rename); err != nil {
		return err
	}
	audit.EmitAction(ctx, audit.DatasourceUpdated, audit.Record{
		WorkspaceID: rename.WorkspaceID.String(),
		TargetID:    rename.ID.String(),
		TargetLabel: rename.Name,
		Status:      audit.StatusSuccess,
		Payload:     map[string]any{"db_type": existing.DbType, "managed": true},
	})
	return nil
}
