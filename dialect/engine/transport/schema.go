package transport

import (
	"context"

	"github.com/selectDb/dialect/core"
)

func (t *HTTPTransport) GetMetadata(
	ctx context.Context,
	workspaceID,
	instanceID string,
	noCache bool,
) (*core.Metadata, error) {
	path := "datasources/" + instanceID + "/schema"
	if noCache {
		path += "?no_cache=true"
	}
	var meta core.Metadata
	if err := t.Fetch(ctx, "GET", path, nil, workspaceHeader(workspaceID), &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
