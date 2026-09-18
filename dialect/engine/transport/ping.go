package transport

import "context"

func (t *HTTPTransport) Ping(
	ctx context.Context,
	workspaceID,
	instanceID string,
	noCache bool,
) error {
	path := "datasources/" + instanceID + "/ping"
	if noCache {
		path += "?no_cache=true"
	}
	return t.Fetch(ctx, "POST", path, nil, workspaceHeader(workspaceID), nil)
}
