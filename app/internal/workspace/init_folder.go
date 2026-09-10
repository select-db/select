package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"path/filepath"

	"selectDb/internal/api"
	"selectDb/internal/graph"
	"selectDb/internal/server"
)

type createWorkspaceResponse struct {
	ID                string `json:"id"`
	WorkspaceToUserID string `json:"workspace_to_user_id"`
	Name              string `json:"name"`
	OwnerID           string `json:"owner_id"`
}

// InitWorkspaceInFolder creates the workspace on the server, writes the config
// that names it, and opens it. An empty name defaults to the folder's.
func (w *Workspace) InitWorkspaceInFolder(path, name string) (OpenFolderResult, error) {
	folder, err := normalizeFolder(path)
	if err != nil {
		return OpenFolderResult{}, err
	}

	ctx := context.Background()

	u, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OpenFolderResult{}, fmt.Errorf("no current user")
		}
		return OpenFolderResult{}, fmt.Errorf("get current user: %w", err)
	}

	currentServer, err := server.ReadCurrentDomain()
	if err != nil {
		return OpenFolderResult{}, fmt.Errorf("read current server: %w", err)
	}
	if currentServer == "" {
		return OpenFolderResult{}, fmt.Errorf("no current server")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(folder)
	}

	var resp createWorkspaceResponse
	if err := api.Fetch(ctx, "POST", "workspaces", map[string]string{"name": name}, nil, &resp); err != nil {
		return OpenFolderResult{}, fmt.Errorf("create workspace on server: %w", err)
	}

	ws, err := w.CreateWorkspace(CreateWorkspaceParams{
		ID:                resp.ID,
		WorkspaceToUserID: resp.WorkspaceToUserID,
		UserID:            u.ID,
		Name:              resp.Name,
	})
	if err != nil {
		return OpenFolderResult{}, fmt.Errorf("create workspace locally: %w", err)
	}

	// Written last: it is what makes the folder a workspace, and a config for a
	// workspace that was never created is the state the init screen cleans up.
	if err := graph.WriteWorkspaceConfig(folder, currentServer, ws.ID); err != nil {
		return OpenFolderResult{}, err
	}

	if err := w.adoptFolder(ws.ID, folder); err != nil {
		return OpenFolderResult{}, err
	}

	return OpenFolderResult{
		State:         OpenFolderOpened,
		Path:          folder,
		CurrentServer: currentServer,
	}, nil
}
