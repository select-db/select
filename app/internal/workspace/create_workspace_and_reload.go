package workspace

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/internal/api"
)

type createWorkspaceResponse struct {
	ID                string `json:"id"`
	WorkspaceToUserID string `json:"workspace_to_user_id"`
	Name              string `json:"name"`
	OwnerID           string `json:"owner_id"`
}

func (w *Workspace) CreateWorkspaceAndReload(name string) error {
	ctx := context.Background()

	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no current user")
		}
		return fmt.Errorf("get current user: %w", err)
	}

	var resp createWorkspaceResponse
	if err := api.Fetch(ctx, "POST", "workspaces", map[string]string{"name": name}, nil, &resp); err != nil {
		return fmt.Errorf("create workspace on server: %w", err)
	}

	if _, err := w.CreateWorkspace(CreateWorkspaceParams{
		ID:                resp.ID,
		WorkspaceToUserID: resp.WorkspaceToUserID,
		UserID:            u.ID,
		Name:              resp.Name,
	}); err != nil {
		return fmt.Errorf("create workspace locally: %w", err)
	}

	if err := w.EnsureWorkspaceFolderByID(resp.ID, ""); err != nil {
		return fmt.Errorf("ensure workspace folder: %w", err)
	}

	if h := w.ReloadHooks; h != nil {
		if h.BuildWorkspaceGraph != nil {
			if err := h.BuildWorkspaceGraph(); err != nil {
				return fmt.Errorf("build workspace graph: %w", err)
			}
		}
		if h.EmitWorkspaceGraphUpdated != nil {
			h.EmitWorkspaceGraphUpdated()
		}
	}

	return nil
}
