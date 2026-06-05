package git

import (
	"fmt"
	"strconv"
	"strings"
)

// GitFileStatusItem represents the status of a single file in the git working tree.
type GitFileStatusItem struct {
	Path          string `json:"path"`          // Relative path from workspace root
	Status        string `json:"status"`        // "staged", "unstaged", "untracked"
	PorcelainCode string `json:"porcelainCode"` // Git porcelain format code (e.g., "M ", " M", "??", "A ")
}

// GitFileStatus provides file-level git status information for the workspace.
type GitFileStatus struct {
	Branch        string              `json:"branch"`
	Staged        []GitFileStatusItem `json:"staged"`
	Unstaged      []GitFileStatusItem `json:"unstaged"`
	Untracked     []GitFileStatusItem `json:"untracked"`
	HasChanges    bool                `json:"hasChanges"`
	CommitsAhead  int                 `json:"commitsAhead"`
	CommitsBehind int                 `json:"commitsBehind"`
}

// GetGitFileStatus returns file-level git status for the workspace, including
// staged, unstaged, and untracked files.
func (g *Git) GetGitFileStatus() (*GitFileStatus, error) {
	ctx := g.context()

	root, err := g.prepareGitLocal(ctx)
	if err != nil {
		return nil, err
	}

	status := &GitFileStatus{
		Staged:    []GitFileStatusItem{},
		Unstaged:  []GitFileStatusItem{},
		Untracked: []GitFileStatusItem{},
	}

	hasCommits, _ := hasAnyCommits(ctx, root)

	if !hasCommits {
		// No local commits, check if the remote has any so we can report CommitsBehind.
		remoteBranches, err := runGitWithOutput(ctx, root, "branch", "-r")
		if err == nil && strings.TrimSpace(remoteBranches) != "" {
			if defaultBranch, err := remoteDefaultBranch(ctx, root); err == nil && defaultBranch != "" {
				status.Branch = defaultBranch
				if revListOut, err := runGitWithOutput(ctx, root, "rev-list", "--count", "origin/"+defaultBranch); err == nil {
					if count, err := strconv.Atoi(strings.TrimSpace(revListOut)); err == nil {
						status.CommitsBehind = count
					}
				}
			}
		}
	} else {
		branchOut, err := runGitWithOutput(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
		if err == nil {
			branch := strings.TrimSpace(branchOut)
			if branch != "" && branch != "HEAD" {
				status.Branch = branch

				// Compute ahead/behind only when a remote tracking branch exists.
				remoteBranchOut, err := runGitWithOutput(ctx, root, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
				if err == nil && strings.TrimSpace(remoteBranchOut) != "" {
					// Fetch to get accurate counts; errors are silently ignored.
					_ = runGit(ctx, root, "fetch", "origin")

					revListOut, err := runGitWithOutput(ctx, root, "rev-list", "--left-right", "--count", branch+"...@{upstream}")
					if err == nil {
						parts := strings.Fields(strings.TrimSpace(revListOut))
						if len(parts) == 2 {
							if ahead, err := strconv.Atoi(parts[0]); err == nil {
								status.CommitsAhead = ahead
							}
							if behind, err := strconv.Atoi(parts[1]); err == nil {
								status.CommitsBehind = behind
							}
						}
					}
				}
			}
		}
	}

	// Parse porcelain status.
	// XY format: X = index (staged), Y = working tree (unstaged).
	// Leading spaces are significant, do not trim the output.
	output, err := runGitWithOutput(ctx, root, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}

	output = strings.TrimRight(output, "\n\r")
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 3 {
			continue
		}

		indexStatus := line[0]
		workTreeStatus := line[1]
		pathPart := strings.TrimSpace(line[2:])
		if pathPart == "" {
			continue
		}

		// Renamed files: "old -> new", use the destination path.
		var path string
		if indexStatus == 'R' || workTreeStatus == 'R' {
			if arrowIdx := strings.Index(pathPart, " -> "); arrowIdx >= 0 {
				path = strings.TrimSpace(pathPart[arrowIdx+4:])
			} else {
				path = pathPart
			}
		} else {
			path = pathPart
		}

		// Unquote paths with spaces.
		if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
			path = path[1 : len(path)-1]
		}

		// Skip directory entries (trailing slash).
		if strings.HasSuffix(path, "/") {
			continue
		}

		porcelainCode := line[:2]

		if indexStatus == '?' && workTreeStatus == '?' {
			status.Untracked = append(status.Untracked, GitFileStatusItem{path, "untracked", porcelainCode})
			status.HasChanges = true
			continue
		}

		if indexStatus != ' ' && indexStatus != '?' {
			status.Staged = append(status.Staged, GitFileStatusItem{path, "staged", porcelainCode})
			status.HasChanges = true
		}

		if workTreeStatus != ' ' && workTreeStatus != '?' {
			status.Unstaged = append(status.Unstaged, GitFileStatusItem{path, "unstaged", porcelainCode})
			status.HasChanges = true
		}
	}

	return status, nil
}
