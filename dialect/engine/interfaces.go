package engine

import (
	"context"
	"database/sql"

	"github.com/selectDb/dialect/core"
)

// Conn is an open database and the rules a statement on it is checked against.
// A proxied datasource passes an empty Conn: the remote server holds all of it.
type Conn struct {
	DB    *sql.DB
	Meta  *core.Metadata
	Perms core.CompiledPermissions
	// Prepare, when set, runs on the connection a statement is about to run on,
	// for settings a driver keeps per connection. An error refuses the statement.
	Prepare func(c *sql.Conn, statement string) error
}

// QueryRunsAll marks a driver whose Query runs every statement, rows or not.
// The engine never retries its failed query as an exec: part of it may have run.
type QueryRunsAll interface{ QueryRunsAll() }

// querier is what a statement runs on: the pool, or one connection.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// acquire returns what statement runs on. Call release once its rows are closed.
//   - no Prepare: the pool
//   - Prepare: one connection, which Prepare has set up for statement
func (c Conn) acquire(ctx context.Context, statement string) (q querier, release func(), err error) {
	if c.Prepare == nil {
		return c.DB, func() {}, nil
	}
	conn, err := c.DB.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := c.Prepare(conn, statement); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, func() { _ = conn.Close() }, nil
}

// RowStream reads streamed query results.
type RowStream interface {
	Columns() (cols []string, err error)
	Next() (values []any, ok bool, err error)
	Summary() (rowCount, affected int64, durationMs int64, err error)
	Close() error
}

// RowSink receives a query's result: OnColumns, OnRow for each row, then
// OnDone, or OnError at the first failure.
type RowSink interface {
	OnColumns(cols []string) error
	OnRow(values []any) error
	OnDone(rowCount, affected, durationMs int64) error
	OnError(err error)
}

// Optional RowSink hooks, called when the sink implements them.
type (
	// executedSink learns how long the statement took, before its rows stream.
	executedSink interface{ OnExecuted(durationMs int64) }
	// columnTypesSink learns the driver's column types.
	columnTypesSink interface{ SetColumnTypes([]*sql.ColumnType) }
	// truncatedSink learns that Options.MaxRows cut the result short.
	truncatedSink interface{ OnTruncated() }
)
