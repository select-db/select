package git

import (
	"context"
)

// GitWorkspaceStatus describes the Git-level state of a workspace on disk. This is the
// primary shape that the frontend will consume to decide which CTAs to show
// (init, publish, link, etc).
type GitWorkspaceStatus struct {
	GitAvailable bool   `json:"gitAvailable"`
	IsGitRepo    bool   `json:"isGitRepo"`
	HasRemote    bool   `json:"hasRemote"`
	RemoteURL    string `json:"remoteUrl,omitempty"`
	// ConfiguredRemoteUrl is the workspace's server-authoritative git remote,
	// read from the DB independently of git availability. When git is missing
	// this is the only way the UI can tell the user the workspace is supposed
	// to be linked (and therefore its files cannot sync until git is installed).
	ConfiguredRemoteUrl string `json:"configuredRemoteUrl,omitempty"`
}

// GetGitWorkspaceStatus inspects the local filesystem and returns high-level Git status information.
//
// It is intentionally conservative: failure to run git commands is treated as
// "not a git repo" / "no remote" rather than an error, so that the UI can
// still render and offer init/publish flows.
func (g *Git) GetGitWorkspaceStatus() (*GitWorkspaceStatus, error) {
	ctx := context.Background()

	stat := &GitWorkspaceStatus{}

	if g.Graph == nil || g.Graph.WorkspaceGraph == nil {
		return stat, nil
	}
	workspaceID := g.Graph.WorkspaceGraph.ID

	stat.ConfiguredRemoteUrl = g.configuredRemote(ctx, workspaceID)

	root, err := g.workspaceRootPath(workspaceID)
	if err != nil {
		return stat, err
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

// configuredRemote reads the workspace's git_remote_url from the DB. It never
// errors or panics: this status call is intentionally conservative so the UI
// can always render.
func (g *Git) configuredRemote(ctx context.Context, workspaceID string) (url string) {
	defer func() { _ = recover() }()
	if g.Queries == nil {
		return ""
	}
	ws, err := g.Queries.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return ""
	}
	return ws.GitRemoteUrl.Or("")
}
