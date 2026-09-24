package engine

import (
	"context"
	"database/sql"

	"github.com/selectDb/dialect/core"
)

// Conn holds DB handle + metadata + permissions for local query execution.
// Empty Conn{} for proxified queries (remote handles everything).
type Conn struct {
	DB    *sql.DB
	Meta  *core.Metadata
	Perms core.CompiledPermissions
	// Prepare, when set, runs on the connection taken for each user statement,
	// before it. For settings a driver keeps per connection; an error aborts.
	Prepare func(*sql.Conn) error
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// statementConn is where one user statement runs: the pool, or a connection
// Prepare has set up. Call release once the statement's rows are closed.
func (c Conn) statementConn(ctx context.Context) (q querier, release func(), err error) {
	if c.Prepare == nil {
		return c.DB, func() {}, nil
	}
	sc, err := c.DB.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := c.Prepare(sc); err != nil {
		_ = sc.Close()
		return nil, nil, err
	}
	return sc, func() { _ = sc.Close() }, nil
}

// RowStream reads streamed query results.
type RowStream interface {
	Columns() (cols []string, err error)
	Next() (values []any, ok bool, err error)
	Summary() (rowCount, affected int64, durationMs int64, err error)
	Close() error
}

// RowSink writes streamed query results.
type RowSink interface {
	OnColumns(cols []string) error
	OnRow(values []any) error
	OnDone(rowCount, affected, durationMs int64) error
	OnError(err error)
}
