package git

import (
	"fmt"
	"os"
	"path/filepath"
)

const maxDiffFileBytes = 1 << 20 // 1 MiB

// GetFileDiffContentParams defines the input for fetching diff content for a file.
type GetFileDiffContentParams struct {
	Path string `json:"path"` // Relative path from workspace root
}

// GetFileDiffContentResult holds the left (HEAD) and right (working tree) content.
type GetFileDiffContentResult struct {
	LeftContent  string `json:"leftContent"`
	RightContent string `json:"rightContent"`
}

// GetFileDiffContent returns the content of a file at HEAD (left) and in the working tree (right).
// Left is empty for untracked/new files; right is empty for deleted files.
// Files larger than 1 MiB return a placeholder string instead of their content.
func (g *Git) GetFileDiffContent(params GetFileDiffContentParams) (*GetFileDiffContentResult, error) {
	ctx := g.context()

	root, err := g.workspaceRootPath(g.Graph.WorkspaceGraph.ID)
	if err != nil {
		return nil, fmt.Errorf("workspace root: %w", err)
	}

	ok, err := isGitRepo(ctx, root)
	if err != nil || !ok {
		return nil, fmt.Errorf("workspace is not a git repository")
	}

	pathOS := filepath.FromSlash(params.Path)
	pathGit := filepath.ToSlash(params.Path)
	fullPath := filepath.Join(root, pathOS)

	result := &GetFileDiffContentResult{}

	// Left: content at HEAD (empty if new/untracked).
	if runGit(ctx, root, "rev-parse", "--verify", "HEAD") == nil {
		out, err := runGitWithOutput(ctx, root, "show", "HEAD:"+pathGit)
		if err == nil {
			if len(out) > maxDiffFileBytes {
				result.LeftContent = "(file too large to display)"
			} else {
				result.LeftContent = out
			}
		}
	}

	// Right: working tree content (empty if deleted).
	info, err := os.Stat(fullPath)
	if err == nil {
		if info.Size() > maxDiffFileBytes {
			result.RightContent = "(file too large to display)"
		} else {
			data, err := os.ReadFile(fullPath)
			if err == nil {
				result.RightContent = string(data)
			}
		}
	}

	return result, nil
}
