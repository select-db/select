package workspace

import (
	"context"
	"fmt"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/audit"
	"backend/internal/syncer/types"
)

// ApplyDelete sets deleted_at and updated_at for the workspace identified by the commit.
// The commit ObjectID is the workspace id.
func ApplyDelete(ctx context.Context, userID string, c types.Commit) (bool, *types.RestoredItem, error) {
	id := c.ObjectID
	if id == "" {
		id = c.WorkspaceID
	}
	if id == "" {
		return false, nil, fmt.Errorf("workspace: missing id")
	}
	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("workspace: invalid id %q: %w", id, err)
	}
	// only the owner may delete a workspace; the REST handler enforces this and
	// the sync path must too, or any member with settings.write can wipe it
	ownerID, err := db.Queries.GetWorkspaceOwnerID(ctx, idUUID)
	if err != nil {
		return false, nil, fmt.Errorf("workspace: lookup owner: %w", err)
	}
	if ownerID.String() != userID {
		return false, nil, fmt.Errorf("workspace: only the owner can delete the workspace")
	}
	if err := db.Queries.SetWorkspaceDeletedAt(ctx, idUUID); err != nil {
		return false, nil, fmt.Errorf("workspace: set deleted_at: %w", err)
	}
	audit.EmitChange(ctx, audit.WorkspaceDeleted, id, id, nil, nil)
	return true, nil, nil
}
