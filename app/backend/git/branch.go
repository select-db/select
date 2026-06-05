package git

import (
	"fmt"
	"strings"
)

// BranchInfo represents information about a git branch
type BranchInfo struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"isCurrent"`
	IsRemote  bool   `json:"isRemote"`
}

// GetBranches returns a list of all branches (local and remote).
func (g *Git) GetBranches() ([]BranchInfo, error) {
	ctx := g.context()

	root, err := g.prepareGit(ctx)
	if err != nil {
		return nil, err
	}

	var branches []BranchInfo

	hasCommits, _ := hasAnyCommits(ctx, root)

	if !hasCommits {
		// No local commits yet, only show remote branches.
		remoteBranches, err := runGitWithOutput(ctx, root, "branch", "-r")
		if err == nil && strings.TrimSpace(remoteBranches) != "" {
			for _, line := range strings.Split(strings.TrimSpace(remoteBranches), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.Contains(line, "->") {
					continue
				}
				branches = append(branches, BranchInfo{
					Name:     strings.TrimPrefix(line, "origin/"),
					IsRemote: true,
				})
			}
		}
		return branches, nil
	}

	// Local branches.
	localOut, err := runGitWithOutput(ctx, root, "branch")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(localOut), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			isCurrent := strings.HasPrefix(line, "* ")
			branches = append(branches, BranchInfo{
				Name:      strings.TrimPrefix(line, "* "),
				IsCurrent: isCurrent,
			})
		}
	}

	// Remote-only branches (deduplicated against locals).
	localSet := make(map[string]bool, len(branches))
	for _, b := range branches {
		localSet[b.Name] = true
	}

	remoteOut, err := runGitWithOutput(ctx, root, "branch", "-r")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(remoteOut), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "->") {
				continue
			}
			branchName := strings.TrimPrefix(line, "origin/")
			if !localSet[branchName] {
				branches = append(branches, BranchInfo{Name: branchName, IsRemote: true})
			}
		}
	}

	return branches, nil
}

// SwitchBranchParams defines the input for switching branches.
type SwitchBranchParams struct {
	BranchName string `json:"branchName"`
}

// SwitchBranch switches to the specified branch, creating it from remote if needed.
func (g *Git) SwitchBranch(params SwitchBranchParams) error {
	ctx := g.context()

	root, err := g.prepareGit(ctx)
	if err != nil {
		return err
	}

	hasCommits, _ := hasAnyCommits(ctx, root)

	if !hasCommits {
		if err := runGit(ctx, root, "checkout", "-b", params.BranchName, "origin/"+params.BranchName); err != nil {
			return fmt.Errorf("failed to checkout remote branch: %w", err)
		}
		return nil
	}

	// Local branch exists?
	localBranches, err := runGitWithOutput(ctx, root, "branch", "--list", params.BranchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if strings.TrimSpace(localBranches) != "" {
		if err := runGit(ctx, root, "checkout", params.BranchName); err != nil {
			return fmt.Errorf("failed to switch branch: %w", err)
		}
		return nil
	}

	// Remote branch exists?
	remoteBranches, err := runGitWithOutput(ctx, root, "branch", "-r", "--list", "origin/"+params.BranchName)
	if err != nil {
		return fmt.Errorf("failed to check remote branches: %w", err)
	}

	if strings.TrimSpace(remoteBranches) != "" {
		if err := runGit(ctx, root, "checkout", "-b", params.BranchName, "origin/"+params.BranchName); err != nil {
			return fmt.Errorf("failed to create tracking branch: %w", err)
		}
	} else {
		if err := runGit(ctx, root, "checkout", "-b", params.BranchName); err != nil {
			return fmt.Errorf("failed to create new branch: %w", err)
		}
	}

	return nil
}
