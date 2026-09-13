package workspace

import (
	"context"
	"fmt"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

// ApplyDelete reports whether the deleted workspace was the open one.
func ApplyDelete(ctx context.Context, queries *generated.Queries, payload map[string]any) (wasCurrent bool, err error) {
	workspaceID := utils.MapGetString(payload, "id")
	if workspaceID == "" {
		return false, nil
	}

	currentWTU, _ := queries.GetCurrentWorkspaceToUser(ctx)
	wasCurrent = currentWTU.WorkspaceID == workspaceID

	if err := queries.DeleteWorkspaceToUserByWorkspaceID(ctx, workspaceID); err != nil {
		return false, fmt.Errorf("delete workspace_to_user by workspace_id: %w", err)
	}
	if err := queries.DeleteWorkspaceByID(ctx, workspaceID); err != nil {
		return false, fmt.Errorf("delete workspace: %w", err)
	}
	return wasCurrent, nil
}
