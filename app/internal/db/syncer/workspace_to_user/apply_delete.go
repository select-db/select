package workspace_to_user

import (
	"context"
	"fmt"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

// ApplyDelete reports whether the revoked membership held the open folder.
func ApplyDelete(ctx context.Context, queries *generated.Queries, payload map[string]any) (wasCurrent bool, err error) {
	id := utils.MapGetString(payload, "id")
	workspaceID := utils.MapGetString(payload, "workspace_id")
	if id == "" || workspaceID == "" {
		return false, nil
	}

	currentWTU, _ := queries.GetCurrentWorkspaceToUser(ctx)
	wasCurrent = currentWTU.ID == id && currentWTU.WorkspaceID == workspaceID

	if err := queries.DeleteWorkspaceToUserByID(ctx, generated.DeleteWorkspaceToUserByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	}); err != nil {
		return false, fmt.Errorf("delete workspace_to_user: %w", err)
	}
	return wasCurrent, nil
}
