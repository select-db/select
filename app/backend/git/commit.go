package git

import (
	"fmt"
)

// CommitParams defines the input for creating a commit.
type CommitParams struct {
	Message string `json:"message"`
}

// CommitChanges creates a commit with the given message from all staged changes.
func (g *Git) CommitChanges(params CommitParams) error {
	ctx := g.context()

	root, err := g.prepareGitLocal(ctx)
	if err != nil {
		return err
	}

	if params.Message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	if err := runGit(ctx, root, "commit", "-m", params.Message); err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	return nil
}
