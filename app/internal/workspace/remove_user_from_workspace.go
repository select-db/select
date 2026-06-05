package workspace

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/internal/db/generated"
)

func (w *Workspace) RemoveUserFromWorkspace(userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	ctx := context.Background()

	wtu, err := w.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no current workspace")
		}
		return fmt.Errorf("get current workspace: %w", err)
	}

	ownerID, err := w.Queries.GetWorkspaceOwnerID(ctx, wtu.WorkspaceID)
	if err != nil {
		return fmt.Errorf("get workspace owner: %w", err)
	}
	if ownerID.String == userID {
		return fmt.Errorf("cannot remove workspace owner")
	}

	// Remove all role assignments for this user in the workspace
	roleIDs, err := w.Queries.ListUserToRoleIDsByUserAndWorkspace(ctx, generated.ListUserToRoleIDsByUserAndWorkspaceParams{
		UserID:      userID,
		WorkspaceID: wtu.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("list user roles: %w", err)
	}
	for _, id := range roleIDs {
		if err := w.Queries.RemoveUserFromRole(ctx, id); err != nil {
			return fmt.Errorf("remove role assignment: %w", err)
		}
	}

	// Look up workspace_to_user ID, then delete by ID so the interceptor can track it
	targetWtu, err := w.Queries.GetWorkspaceToUserByUserAndWorkspace(ctx, generated.GetWorkspaceToUserByUserAndWorkspaceParams{
		UserID:      userID,
		WorkspaceID: wtu.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("find workspace membership: %w", err)
	}
	if err := w.Queries.DeleteWorkspaceToUserTracked(ctx, targetWtu.ID); err != nil {
		return fmt.Errorf("remove user from workspace: %w", err)
	}

	return nil
}
