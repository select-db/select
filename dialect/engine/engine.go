// Package engine is the desktop app's entry point: Client sends each call to
// the local subpackages (connect, query, results, schema) or, for a proxied
// datasource, to the backend through Transport. The backend uses them directly.
package engine

import (
	"context"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine/query"
)

// Transport routes proxified operations to the remote backend.
// query.Cancel is handled via context propagation (HTTP disconnect → server ctx cancelled).
type Transport interface {
	OpenStream(
		ctx context.Context,
		workspaceID,
		instanceID,
		dbType,
		sql string,
		opts query.Options,
	) (query.RowStream, error)

	GetMetadata(
		ctx context.Context,
		workspaceID,
		instanceID string,
		noCache bool,
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
