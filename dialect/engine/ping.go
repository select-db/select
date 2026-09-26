package engine

import (
	"context"

	"github.com/selectDb/dialect/engine/query"
)

// Ping checks that conn.DB is reachable. For proxified instances the
// Client routes this through Transport instead (Phase B); this function
// handles the local path only.
func Ping(ctx context.Context, conn query.Conn) error {
	return conn.DB.PingContext(ctx)
}
