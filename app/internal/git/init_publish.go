package git

import (
	"context"
	"fmt"
	"os"

	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
)

// InitAndPublishParams defines the input for initializing a local git repo for
// a workspace and preparing it for publication to GitHub. The actual GitHub
// repository creation will be delegated to the remote backend in a later step.
type InitAndPublishParams struct {
	RepoName   string `json:"repoName"`
	Visibility string `json:"visibility"` // "public" or "private"
	RemoteURL  string `json:"remoteUrl,omitempty"`
}

// InitAndPublishRepo performs local git initialization and creates an initial
// commit for the given workspace. A later iteration will extend this to call
// the remote backend to actually create the GitHub repo and push.
func (g *Git) InitAndPublishRepo(params InitAndPublishParams) error {
	root, err := g.workspaceRootPath(g.Graph.WorkspaceGraph.ID)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("ensure workspace root: %w", err)
	}

	ctx := g.context()

	if ok, _ := isGitAvailable(ctx); !ok {
		return fmt.Errorf("git is not installed or not available in PATH")
	}

	if err := ensureGitRepo(ctx, root); err != nil {
		return err
	}

	// Stage all files and create an initial commit if there is anything to
	// commit. This mirrors the behaviour of VS Code on first publish.
	if err := ensureInitialCommit(ctx, root); err != nil {
		return err
	}

	// If a remote URL was provided, configure "origin" and persist to local DB
	// so the syncer creates a commit and the backend gets the update.
	if params.RemoteURL != "" {
		if err := setOrUpdateOrigin(ctx, root, params.RemoteURL); err != nil {
			return fmt.Errorf("failed to configure git remote: %w", err)
		}
		if err := g.Queries.UpdateWorkspaceGitRemote(ctx, generated.UpdateWorkspaceGitRemoteParams{
			ID:           g.Graph.WorkspaceGraph.ID,
			GitRemoteUrl: db_types.NewJSONNullString(params.RemoteURL),
		}); err != nil {
			return fmt.Errorf("failed to persist git remote url: %w", err)
		}
	}

	// TODO: Call remote backend to create GitHub repo (if needed) and push.

	return nil
}

func ensureInitialCommit(ctx context.Context, dir string) error {
	if err := runGit(ctx, dir, "add", "."); err != nil {
		return err
	}

	out, err := runGitWithOutput(ctx, dir, "status", "--porcelain")
	if err != nil {
		return err
	}
	if len(out) == 0 {
		return nil
	}

	return runGit(ctx, dir, "commit", "-m", "Initial commit")
}
