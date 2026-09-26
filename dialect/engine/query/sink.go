package query

import "database/sql"

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
	// ExecutedSink learns how long the statement took, before its rows stream.
	ExecutedSink interface{ OnExecuted(durationMs int64) }
	// ColumnTypesSink learns the driver's column types.
	ColumnTypesSink interface{ SetColumnTypes([]*sql.ColumnType) }
	// TruncatedSink learns that Options.MaxRows cut the result short.
	TruncatedSink interface{ OnTruncated() }
)
