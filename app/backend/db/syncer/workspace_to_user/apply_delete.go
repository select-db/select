package workspace_to_user

import (
	"context"
	"fmt"

	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
)

func ApplyDelete(ctx context.Context, queries *generated.Queries, payload map[string]any) (wasCurrent bool, userIDForSwitch string, err error) {
	id := utils.MapGetString(payload, "id")
	workspaceID := utils.MapGetString(payload, "workspace_id")
	userIDForSwitch = utils.MapGetString(payload, "user_id")
	if id == "" || workspaceID == "" {
		return false, "", nil
	}

	currentWTU, _ := queries.GetCurrentWorkspaceToUser(ctx)
	if userIDForSwitch == "" {
		userIDForSwitch = currentWTU.UserID
	}
	wasCurrent = currentWTU.ID == id && currentWTU.WorkspaceID == workspaceID

	if err := queries.DeleteWorkspaceToUserByID(ctx, generated.DeleteWorkspaceToUserByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	}); err != nil {
		return false, "", fmt.Errorf("delete workspace_to_user: %w", err)
	}
	return wasCurrent, userIDForSwitch, nil
}
