package git

import (
	"context"
	"fmt"

	"selectDb/backend/db/db_types"
	"selectDb/backend/db/generated"
)

type LinkExistingParams struct {
	RemoteURL string `json:"remoteUrl"`
}

type LinkStatus struct {
	Scenario string `json:"scenario"`
	Branch   string `json:"branch"`
}

// LinkExistingRepo is the user-facing Wails method. It performs all
// non-destructive setup (init, remote, persist, fetch) and returns a
// LinkStatus. When Scenario is "checkout", the frontend shows a choice modal
// and then calls CompleteLinkExistingRepo with the user's decision.
func (g *Git) LinkExistingRepo(params LinkExistingParams) (LinkStatus, error) {
	workspaceID := g.Graph.WorkspaceGraph.ID
	root, err := g.workspaceRootPath(workspaceID)
	if err != nil {
		return LinkStatus{}, err
	}

	ctx := g.context()

	if ok, _ := isGitAvailable(ctx); !ok {
		return LinkStatus{}, fmt.Errorf("git is not installed or not available in PATH")
	}

	if err := ensureGitRepo(ctx, root); err != nil {
		return LinkStatus{}, err
	}

	if err := setOrUpdateOrigin(ctx, root, params.RemoteURL); err != nil {
		return LinkStatus{}, err
	}

	if err := g.Queries.UpdateWorkspaceGitRemote(ctx, generated.UpdateWorkspaceGitRemoteParams{
		ID:           workspaceID,
		GitRemoteUrl: db_types.NewJSONNullString(params.RemoteURL),
	}); err != nil {
		return LinkStatus{}, fmt.Errorf("failed to persist git remote url: %w", err)
	}

	if err := runGit(ctx, root, "fetch", "origin"); err != nil {
		return LinkStatus{}, fmt.Errorf("failed to fetch from remote: %w", err)
	}

	hasCommits, err := hasAnyCommits(ctx, root)
	if err != nil {
		return LinkStatus{}, err
	}

	if !hasCommits {
		branch, err := remoteDefaultBranch(ctx, root)
		if err != nil {
			return LinkStatus{}, err
		}
		if branch != "" {
			// Remote has commits; pause and let the user choose.
			g.pendingLinkWorkspaceID = workspaceID
			g.pendingLinkBranch = branch
			return LinkStatus{Scenario: "checkout", Branch: branch}, nil
		}
		// Remote is also empty — nothing destructive to do.
		if err := writeLinkMarker(ctx, root, params.RemoteURL); err != nil {
			return LinkStatus{}, err
		}
		return LinkStatus{Scenario: "done"}, nil
	}

	// Local has commits: just set up tracking.
	if err := setupBranchTracking(ctx, root); err != nil {
		return LinkStatus{}, err
	}
	if err := writeLinkMarker(ctx, root, params.RemoteURL); err != nil {
		return LinkStatus{}, err
	}
	return LinkStatus{Scenario: "done"}, nil
}

// CompleteLinkExistingRepo finalises a pending link started by LinkExistingRepo.
// choice "checkout" replaces local files with the remote branch.
// choice "keep" leaves local files untouched; the remote is already configured.
func (g *Git) CompleteLinkExistingRepo(choice string) error {
	workspaceID := g.pendingLinkWorkspaceID
	if workspaceID == "" {
		return fmt.Errorf("no pending link to complete")
	}
	g.pendingLinkWorkspaceID = ""
	g.pendingLinkBranch = ""

	ctx := g.context()
	root, err := g.workspaceRootPath(workspaceID)
	if err != nil {
		return err
	}

	if choice != "keep" {
		if err := checkoutRemoteDefaultBranch(ctx, root); err != nil {
			return err
		}
	}

	// Record the materialized remote so the syncer's reconcile treats this
	// machine as already converged and never re-clones the admin's own repo.
	origin, _ := getGitRemoteOrigin(ctx, root)
	if origin != "" {
		return writeLinkMarker(ctx, root, origin)
	}
	return nil
}

// LinkExistingRepoForWorkspace configures the local git repository for the
// given workspace ID to point at an existing remote. It delegates to
// ReconcileWorkspaceRemote so there is a single, idempotent, self-healing code
// path for materializing a workspace's repository.
func (g *Git) LinkExistingRepoForWorkspace(workspaceID string, params LinkExistingParams) error {
	_, err := g.ReconcileWorkspaceRemote(workspaceID, &params.RemoteURL)
	return err
}

func setOrUpdateOrigin(ctx context.Context, dir, remoteURL string) error {
	if err := runGit(ctx, dir, "remote", "add", "origin", remoteURL); err == nil {
		return nil
	}
	if err := runGit(ctx, dir, "remote", "set-url", "origin", remoteURL); err != nil {
		return fmt.Errorf("failed to configure git remote: %w", err)
	}
	return nil
}

// checkoutRemoteDefaultBranch checks out the remote's default branch, replacing
// local untracked files. Used when the local repo has no commits.
func checkoutRemoteDefaultBranch(ctx context.Context, dir string) error {
	branchName, err := remoteDefaultBranch(ctx, dir)
	if err != nil {
		return err
	}
	if branchName == "" {
		return fmt.Errorf("failed to determine remote default branch")
	}

	if err := runGit(ctx, dir, "clean", "-fd"); err != nil {
		return fmt.Errorf("failed to clean untracked files before checkout: %w", err)
	}

	if err := runGit(ctx, dir, "checkout", "-b", branchName, "--track", "origin/"+branchName); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", branchName, err)
	}

	return nil
}

// setupBranchTracking sets up the current branch to track the corresponding remote branch.
// If the remote branch doesn't exist, this is a no-op (user needs to push first).
func setupBranchTracking(ctx context.Context, dir string) error {
	branch, err := getCurrentBranch(ctx, dir)
	if err != nil {
		return err
	}

	remoteBranch := "origin/" + branch
	if _, err = runGitWithOutput(ctx, dir, "rev-parse", "--verify", remoteBranch); err != nil {
		return nil
	}

	if err := runGit(ctx, dir, "branch", "--set-upstream-to="+remoteBranch, branch); err != nil {
		return fmt.Errorf("failed to set up branch tracking: %w", err)
	}

	return nil
}
