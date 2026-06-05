package transport

import (
	"context"

	"github.com/selectDb/dialect/core"
)

type schemaRequest struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
}

func (t *HTTPTransport) GetMetadata(
	ctx context.Context,
	workspaceID,
	instanceID string,
) (*core.Metadata, error) {
	var meta core.Metadata
	if err := t.Fetch(ctx, "POST", "datasource/schema", schemaRequest{
		ID:          instanceID,
		WorkspaceID: workspaceID,
	}, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
