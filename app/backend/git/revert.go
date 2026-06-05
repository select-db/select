package git

import (
	"fmt"
)

// RevertFileParams defines the input for reverting changes to a file.
type RevertFileParams struct {
	Path string `json:"path"` // Relative path from workspace root
}

// RevertFile discards all changes to a file (both staged and unstaged).
// For staged files, it unstages them first, then discards working tree changes.
// For unstaged files, it just discards working tree changes.
// After reverting, it cleans up any empty directories that were left behind
// (e.g., when a renamed file is reverted to its original location).
func (g *Git) RevertFile(params RevertFileParams) error {
	ctx := g.context()

	root, err := g.prepareGitLocal(ctx)
	if err != nil {
		return err
	}

	hasCommits, _ := hasAnyCommits(ctx, root)

	if hasCommits {
		// Unstage and discard working tree changes.
		_ = runGit(ctx, root, "restore", "--staged", params.Path)
		if err := runGit(ctx, root, "restore", params.Path); err != nil {
			return fmt.Errorf("failed to revert file: %w", err)
		}
	} else {
		// No commits: file is untracked, just delete it.
		if err := runGit(ctx, root, "clean", "-fd", "--", params.Path); err != nil {
			return fmt.Errorf("failed to revert file: %w", err)
		}
	}

	return nil
}

// RevertAll discards all changes to all files, including deleting untracked files.
func (g *Git) RevertAll() error {
	ctx := g.context()

	root, err := g.prepareGitLocal(ctx)
	if err != nil {
		return err
	}

	// git restore requires a HEAD commit to exist. When the repo has never been
	// committed, all files are untracked and clean -fd below handles them.
	if hasCommits, _ := hasAnyCommits(ctx, root); hasCommits {
		if err := runGit(ctx, root, "restore", "--staged", "--worktree", "."); err != nil {
			return fmt.Errorf("failed to revert tracked files: %w", err)
		}
	}

	// Remove untracked files and directories.
	if err := runGit(ctx, root, "clean", "-fd"); err != nil {
		return fmt.Errorf("failed to clean untracked files: %w", err)
	}

	return nil
}
