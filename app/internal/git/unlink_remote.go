package git

import (
	"fmt"

	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
)

// UnlinkRemote removes the "origin" remote from the workspace's git repository
// and clears the workspace's stored git_remote_url so the workspace is considered unsynced.
// The local .git directory and commits are left intact.
func (g *Git) UnlinkRemote() error {
	ctx := g.context()

	root, err := g.workspaceRootPath(g.Graph.WorkspaceGraph.ID)
	if err != nil {
		return err
	}

	if ok, _ := isGitAvailable(ctx); !ok {
		return fmt.Errorf("git is not installed or not available in PATH")
	}

	isRepo, _ := isGitRepo(ctx, root)
	if !isRepo {
		return fmt.Errorf("workspace is not a git repository")
	}

	remote, _ := getGitRemoteOrigin(ctx, root)
	if remote == "" {
		// Already no remote; clear DB in case it was out of sync.
		_ = g.Queries.UpdateWorkspaceGitRemote(ctx, generated.UpdateWorkspaceGitRemoteParams{
			ID:           g.Graph.WorkspaceGraph.ID,
			GitRemoteUrl: db_types.JSONNullString{},
		})
		return nil
	}

	if err := runGit(ctx, root, "remote", "remove", "origin"); err != nil {
		return fmt.Errorf("failed to remove remote origin: %w", err)
	}

	if err := g.Queries.UpdateWorkspaceGitRemote(ctx, generated.UpdateWorkspaceGitRemoteParams{
		ID:           g.Graph.WorkspaceGraph.ID,
		GitRemoteUrl: db_types.JSONNullString{},
	}); err != nil {
		return fmt.Errorf("failed to clear workspace git remote url: %w", err)
	}

	return nil
}
