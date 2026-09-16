package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// BranchInfo represents information about a git branch
type BranchInfo struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"isCurrent"`
	IsRemote  bool   `json:"isRemote"`
}

// branchLimit caps how many branches of each kind the picker asks git for: a
// repository with thousands of them spent the difference building a menu nobody
// reads to the end. A branch past the cap is still reachable, since SwitchBranch
// takes a name the picker never listed. A var so tests can lower it.
var branchLimit = 100

// branchNames lists the branches under a ref prefix, most recently committed to
// first, at most branchLimit of them. The count is git's, so it stops walking
// rather than handing back every ref for us to drop.
func branchNames(ctx context.Context, root, prefix string) []string {
	out, err := runGitWithOutput(ctx, root,
		"for-each-ref",
		"--sort=-committerdate",
		"--count="+strconv.Itoa(branchLimit),
		"--format=%(refname:short)",
		prefix,
	)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}

	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name := strings.TrimSpace(line)
		// origin/HEAD is the symbolic ref, not a branch to switch to.
		if name == "" || name == "origin/HEAD" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// GetBranches returns the branches to pick from, local ones first, each kind
// capped at branchLimit and ordered by how recently it was committed to.
func (g *Git) GetBranches() ([]BranchInfo, error) {
	ctx := g.context()

	root, err := g.prepareGit(ctx)
	if err != nil {
		return nil, err
	}

	var branches []BranchInfo

	// Empty on an unborn HEAD. A repository with no commits has no local refs
	// either, so the remote loop below is then the whole answer.
	current, _ := getCurrentBranch(ctx, root)

	localSet := make(map[string]bool, branchLimit)
	for _, name := range branchNames(ctx, root, "refs/heads") {
		localSet[name] = true
		branches = append(branches, BranchInfo{Name: name, IsCurrent: name == current})
	}

	// The branch you are on belongs in the list whether or not it is among the
	// most recent: it is the one the picker marks as current.
	if current != "" && !localSet[current] {
		localSet[current] = true
		branches = append([]BranchInfo{{Name: current, IsCurrent: true}}, branches...)
	}

	// Remote-only branches (deduplicated against locals).
	for _, name := range branchNames(ctx, root, "refs/remotes/origin") {
		branchName := strings.TrimPrefix(name, "origin/")
		if !localSet[branchName] {
			branches = append(branches, BranchInfo{Name: branchName, IsRemote: true})
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
