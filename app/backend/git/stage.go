package git

import (
	"fmt"
)

// StageFileParams defines the input for staging a file.
type StageFileParams struct {
	Path string `json:"path"` // Relative path from workspace root
}

// StageFile stages a single file for commit.
func (g *Git) StageFile(params StageFileParams) error {
	ctx := g.context()

	root, err := g.prepareGitLocal(ctx)
	if err != nil {
		return err
	}

	if err := runGit(ctx, root, "add", params.Path); err != nil {
		return fmt.Errorf("failed to stage file: %w", err)
	}

	return nil
}

// UnstageFileParams defines the input for unstaging a file.
type UnstageFileParams struct {
	Path string `json:"path"` // Relative path from workspace root
}

// UnstageFile unstages a single file.
func (g *Git) UnstageFile(params UnstageFileParams) error {
	ctx := g.context()

	root, err := g.prepareGitLocal(ctx)
	if err != nil {
		return err
	}

	hasCommits, _ := hasAnyCommits(ctx, root)

	if hasCommits {
		if err := runGit(ctx, root, "restore", "--staged", params.Path); err != nil {
			return fmt.Errorf("failed to unstage file: %w", err)
		}
	} else {
		// No commits yet, use rm --cached instead of restore.
		if err := runGit(ctx, root, "rm", "--cached", params.Path); err != nil {
			return fmt.Errorf("failed to unstage file: %w", err)
		}
	}

	return nil
}

// StageAll stages all changes (modified, deleted, and untracked files).
func (g *Git) StageAll() error {
	ctx := g.context()

	root, err := g.prepareGitLocal(ctx)
	if err != nil {
		return err
	}

	if err := runGit(ctx, root, "add", "."); err != nil {
		return fmt.Errorf("failed to stage all changes: %w", err)
	}

	return nil
}

// UnstageAll unstages all changes.
func (g *Git) UnstageAll() error {
	ctx := g.context()

	root, err := g.prepareGitLocal(ctx)
	if err != nil {
		return err
	}

	hasCommits, _ := hasAnyCommits(ctx, root)

	if hasCommits {
		if err := runGit(ctx, root, "restore", "--staged", "."); err != nil {
			return fmt.Errorf("failed to unstage all changes: %w", err)
		}
	} else {
		if err := runGit(ctx, root, "rm", "--cached", "-r", "."); err != nil {
			return fmt.Errorf("failed to unstage all changes: %w", err)
		}
	}

	return nil
}
