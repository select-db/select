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
	// Prepare, when set, runs with each user statement on the connection taken
	// for it, before it: for rules a driver keeps per connection, and checks the
	// permissions do not cover. An error refuses the statement.
	Prepare func(c *sql.Conn, statement string) error
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// statementConn is where statement runs: the pool, or a connection Prepare has
// set up. Call release once the statement's rows are closed.
// QueryRunsAll is a driver whose Query runs every statement, rows or not. The
// engine never retries its failed Query as Exec: the statement may have run.
type QueryRunsAll interface{ QueryRunsAll() }

// execFallback reports whether a failed Query may be retried as Exec.
func (c Conn) execFallback() bool {
	_, runsAll := c.DB.Driver().(QueryRunsAll)
	return !runsAll
}

func (c Conn) statementConn(ctx context.Context, statement string) (q querier, release func(), err error) {
	if c.Prepare == nil {
		return c.DB, func() {}, nil
	}
	sc, err := c.DB.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := c.Prepare(sc, statement); err != nil {
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
