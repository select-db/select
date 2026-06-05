package workspace_to_user

import (
	"context"
	"fmt"
	"strings"

	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
)

// setCurrent sets the given workspace as the current one for the user.
func setCurrent(ctx context.Context, queries *generated.Queries, userID, workspaceID string) error {
	if err := queries.ClearCurrentWorkspaceToUser(ctx); err != nil {
		return fmt.Errorf("clear current workspace: %w", err)
	}
	return queries.UpdateCurrentWorkspaceToUser(ctx, generated.UpdateCurrentWorkspaceToUserParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
	})
}

// SetFirstAsCurrent sets the first workspace_to_user row matching userID as the current workspace.
// It tries the response slice first; if no match, it falls back to the DB.
func SetFirstAsCurrent(ctx context.Context, queries *generated.Queries, userID string, wtus []map[string]any) error {
	userID = strings.TrimSpace(userID)

	for _, wtu := range wtus {
		wtuUserID := utils.MapGetString(wtu, "user_id")
		wtuWorkspaceID := utils.MapGetString(wtu, "workspace_id")
		if wtuUserID == "" || wtuWorkspaceID == "" {
			continue
		}
		if strings.TrimSpace(wtuUserID) != userID {
			continue
		}
		return setCurrent(ctx, queries, userID, wtuWorkspaceID)
	}

	workspace, err := queries.GetWorkspaceToUserByUserId(ctx, userID)
	if err != nil {
		return nil
	}
	return setCurrent(ctx, queries, userID, workspace.ID)
}
