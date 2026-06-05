package auth

import (
	"context"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"

	"github.com/google/uuid"
)

// After signup: creates a default workspace and workspace_to_user
// for the user if they have none.
func EnsureDefaultWorkspaceForUser(ctx context.Context, userID string) error {
	if userID == "" {
		return nil
	}
	if db.Queries == nil {
		return nil
	}

	userUUID, err := db_types.NewJSONNullUUIDFromString(userID)
	if err != nil {
		return err
	}

	count, err := db.Queries.CountWorkspaceToUserByUserID(ctx, userUUID)
	if err != nil || count > 0 {
		return err
	}

	workspaceID := db_types.NewJSONNullUUID(uuid.New())
	wtuID := db_types.NewJSONNullUUID(uuid.New())

	if err := db.Queries.InsertDefaultWorkspace(ctx, generated.InsertDefaultWorkspaceParams{
		ID:      workspaceID,
		OwnerID: userUUID,
	}); err != nil {
		return err
	}

	if err := db.Queries.InsertWorkspaceToUser(ctx, generated.InsertWorkspaceToUserParams{
		ID:          wtuID,
		WorkspaceID: workspaceID,
		UserID:      userUUID,
	}); err != nil {
		return err
	}

	return nil
}
