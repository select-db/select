package engine

import (
	"database/sql"

	"github.com/selectDb/dialect/core"
)

// Conn holds DB handle + metadata + permissions for local query execution.
// Empty Conn{} for proxified queries (remote handles everything).
type Conn struct {
	DB    *sql.DB
	Meta  *core.Metadata
	Perms core.CompiledPermissions
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
