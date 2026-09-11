package git

import (
	"context"
)

// GitWorkspaceStatus is read off the folder. The app does not configure the
// repository; whoever cloned it chose the remote.
type GitWorkspaceStatus struct {
	GitAvailable bool   `json:"gitAvailable"`
	IsGitRepo    bool   `json:"isGitRepo"`
	HasRemote    bool   `json:"hasRemote"`
	RemoteURL    string `json:"remoteUrl,omitempty"`
}

// GetGitWorkspaceStatus is intentionally conservative: a failing git command
// reads as "not a repo" rather than an error, so the panel always renders.
func (g *Git) GetGitWorkspaceStatus() (*GitWorkspaceStatus, error) {
	ctx := context.Background()

	stat := &GitWorkspaceStatus{}

	root, err := openWorkspaceRoot()
	if err != nil {
		// No folder open is no repository, which is what this reports.
		return stat, nil
	}

	gitOK, _ := isGitAvailable(ctx)
	stat.GitAvailable = gitOK
	if !gitOK {
		return stat, nil
	}

	// Detect if this is a git repository.
	if isRepo, err := isGitRepo(ctx, root); err == nil && isRepo {
		stat.IsGitRepo = true

		// If it's a repo, try to read the origin remote.
		if remote, err := getGitRemoteOrigin(ctx, root); err == nil && remote != "" {
			stat.HasRemote = true
			stat.RemoteURL = remote
		}
	}

	return stat, nil
}
