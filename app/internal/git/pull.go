package git

import (
	"fmt"
)

// PullWorkspaceRepo pulls the current branch of the workspace repository from
// its "origin" remote.
func (g *Git) PullWorkspaceRepo() error {
	ctx := g.context()

	root, err := g.prepareGit(ctx)
	if err != nil {
		return err
	}

	hasCommits, _ := hasAnyCommits(ctx, root)

	if !hasCommits {
		// Freshly linked repo with no local commits, fetch and pull from default branch.
		if err := runGit(ctx, root, "fetch", "origin"); err != nil {
			return fmt.Errorf("failed to fetch from remote: %w", err)
		}

		defaultBranch, err := remoteDefaultBranch(ctx, root)
		if err != nil {
			return fmt.Errorf("failed to determine remote default branch: %w", err)
		}

		if err := runGit(ctx, root, "pull", "origin", defaultBranch); err != nil {
			return fmt.Errorf("failed to pull from remote branch %s: %w", defaultBranch, err)
		}

		return nil
	}

	branch, err := getCurrentBranch(ctx, root)
	if err != nil {
		return err
	}

	if err := runGit(ctx, root, "pull", "origin", branch); err != nil {
		return err
	}

	return nil
}

// PullWithRebase pulls from origin and rebases local commits on top of the remote branch.
func (g *Git) PullWithRebase() error {
	ctx := g.context()

	root, err := g.prepareGit(ctx)
	if err != nil {
		return err
	}

	hasCommits, _ := hasAnyCommits(ctx, root)
	if !hasCommits {
		return g.PullWorkspaceRepo()
	}

	branch, err := getCurrentBranch(ctx, root)
	if err != nil {
		return err
	}

	if err := runGit(ctx, root, "pull", "--rebase", "origin", branch); err != nil {
		return fmt.Errorf("pull with rebase failed: %w", err)
	}
	return nil
}

// ResetBranchToRemote fetches from origin and resets the current branch to match the remote,
// discarding local commits and uncommitted changes.
func (g *Git) ResetBranchToRemote() error {
	ctx := g.context()

	root, err := g.prepareGit(ctx)
	if err != nil {
		return err
	}

	if err := runGit(ctx, root, "fetch", "origin"); err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	hasCommits, _ := hasAnyCommits(ctx, root)
	if !hasCommits {
		defaultBranch, err := remoteDefaultBranch(ctx, root)
		if err != nil {
			return err
		}
		if err := runGit(ctx, root, "checkout", "-b", defaultBranch, "origin/"+defaultBranch); err != nil {
			return fmt.Errorf("checkout remote branch failed: %w", err)
		}
		return nil
	}

	branch, err := getCurrentBranch(ctx, root)
	if err != nil {
		return err
	}

	if err := runGit(ctx, root, "reset", "--hard", "origin/"+branch); err != nil {
		return fmt.Errorf("reset to remote failed: %w", err)
	}
	return nil
}
