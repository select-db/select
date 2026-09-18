package transport

import "context"

type dumpResponse struct {
	SQL string `json:"sql"`
}

func (t *HTTPTransport) DumpSchema(
	ctx context.Context,
	workspaceID,
	instanceID string,
) (string, error) {
	var resp dumpResponse
	if err := t.Fetch(ctx, "GET", "datasources/"+instanceID+"/dump", nil, workspaceHeader(workspaceID), &resp); err != nil {
		return "", err
	}
	return resp.SQL, nil
}
