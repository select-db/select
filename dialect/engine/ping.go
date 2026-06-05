package engine

import "context"

// Ping checks that conn.DB is reachable. For proxified instances the
// Client routes this through Transport instead (Phase B); this function
// handles the local path only.
func Ping(ctx context.Context, conn Conn) error {
	return conn.DB.PingContext(ctx)
}