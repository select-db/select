package transport

import "context"

type pingRequest struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	NoCache     bool   `json:"no_cache,omitempty"`
}

func (t *HTTPTransport) Ping(
	ctx context.Context,
	workspaceID,
	instanceID string,
	noCache bool,
) error {
	return t.Fetch(ctx, "POST", "datasource/ping", pingRequest{
		ID:          instanceID,
		WorkspaceID: workspaceID,
		NoCache:     noCache,
	}, nil)
}
