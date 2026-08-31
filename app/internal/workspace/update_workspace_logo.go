package workspace

import (
	"context"
	"fmt"

	"selectDb/internal/api"
	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
)

// logoResponse is the server's echo of what it stored: the re-encoded image, not
// the bytes we sent. It is a pointer so a response missing the field reads as
// nil rather than as an empty logo.
type logoResponse struct {
	Logo *string `json:"logo"`
}

// UpdateLogo uploads a workspace logo through the backend, then mirrors what the
// server stored into the local database.
//
// The server is the one that validates and re-encodes the image, so the value
// written locally is the server's, not the caller's. The local write is
// deliberately untracked (@no-track): a tracked UPDATE would queue a mutation
// commit and push the logo back up the sync path, which does not carry the
// column. Teammates receive it on their next pull instead, because the endpoint
// bumps the workspace's updated_at.
func (w *Workspace) UpdateLogo(workspaceID, logo string) error {
	ctx := context.Background()

	var res logoResponse
	if err := api.Fetch(ctx, "PUT", "workspaces/"+workspaceID+"/logo",
		map[string]string{"logo": logo},
		api.WorkspaceHeader(workspaceID), &res); err != nil {
		return fmt.Errorf("update workspace logo on server: %w", err)
	}
	if res.Logo == nil {
		return fmt.Errorf("update workspace logo on server: empty response")
	}

	return w.storeLogoLocally(ctx, workspaceID, *res.Logo)
}

func (w *Workspace) storeLogoLocally(ctx context.Context, workspaceID, logo string) error {
	if err := w.Queries.UpdateWorkspaceLogo(ctx, generated.UpdateWorkspaceLogoParams{
		ID:   workspaceID,
		Logo: db_types.NewJSONNullString(logo),
	}); err != nil {
		return fmt.Errorf("store workspace logo: %w", err)
	}

	// Rebuild so the picker and the settings panel show the new logo now rather
	// than at the next pull.
	if w.ReloadHooks != nil && w.ReloadHooks.BuildWorkspaceGraph != nil {
		if err := w.ReloadHooks.BuildWorkspaceGraph(); err != nil {
			return fmt.Errorf("rebuild workspace graph: %w", err)
		}
		if w.ReloadHooks.EmitWorkspaceGraphUpdated != nil {
			w.ReloadHooks.EmitWorkspaceGraphUpdated()
		}
	}
	return nil
}
