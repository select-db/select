package transport

import "context"

type dumpRequest struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
}

type dumpResponse struct {
	SQL string `json:"sql"`
}

func (t *HTTPTransport) DumpSchema(
	ctx context.Context,
	workspaceID,
	instanceID string,
) (string, error) {
	var resp dumpResponse
	if err := t.Fetch(ctx, "POST", "datasource/dump", dumpRequest{
		ID:          instanceID,
		WorkspaceID: workspaceID,
	}, &resp); err != nil {
		return "", err
	}
	return resp.SQL, nil
}
