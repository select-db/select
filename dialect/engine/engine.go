// Package engine: dialect-aware query layer shared by app and remote backend.
// App uses Client (routes local vs proxified). Backend calls package funcs directly.
package engine

import (
	"context"

	"github.com/selectDb/dialect/core"
)

// Transport routes proxified operations to the remote backend.
// Cancel is handled via context propagation (HTTP disconnect → server ctx cancelled).
type Transport interface {
	OpenStream(
		ctx context.Context,
		workspaceID,
		instanceID,
		dbType,
		sql string,
		opts Options,
	) (RowStream, error)

	GetMetadata(
		ctx context.Context,
		workspaceID,
		instanceID string,
	) (*core.Metadata, error)

	Ping(
		ctx context.Context,
		workspaceID,
		instanceID string,
		noCache bool,
	) error

	DumpSchema(
		ctx context.Context,
		workspaceID,
		instanceID string,
	) (string, error)
}
