package engine

import "time"

// DBInstance describes a query target. Proxified routes through Transport.
type DBInstance struct {
	ID        string
	DBType    string
	Proxified bool
}

// Options tunes a single execution.
type Options struct {
	// Timeout is the engine-imposed deadline. 0 means the caller
	// controls cancellation via the context only.
	Timeout time.Duration

	// MaxRows stops the scan loop once this many rows have been emitted.
	// 0 means unlimited (still bounded by the caller's context).
	MaxRows int

	// PageSize is informational only. The engine never truncates;
	// the app's adapter slices the first page from the buffered Result.
	PageSize int

	// ForExport disables result-caching at the call site (full result
	// is returned but not stored for pagination).
	ForExport bool

	// MaxBytes kills the scan loop once this many bytes have been streamed.
	// 0 = unlimited. Converted from max_result_size_mb in the workspace config.
	// The engine enforces a hard ceiling of maxResultSizeHardCap regardless of this value.
	MaxBytes int64
}

// maxResultSizeHardCap is the absolute ceiling enforced by the engine
// regardless of what callers pass in MaxBytes. Prevents accidental or
// malicious bypasses of the workspace config cap.
const maxResultSizeHardCap = 250 * 1024 * 1024 // 250MB

// ColumnEditMeta is the dialect-side, transport-neutral view of per-column
// editability. The app converts these into its own graph.ColumnMetadata
// before sending to the frontend.
type ColumnEditMeta struct {
	HasAllPrimaryKeys  bool
	IsPrimaryKey       bool
	IsForeignKey       bool
	DatabaseID         string
	Schema             string
	Table              string
	OriginalColumnName string
	PrimaryKeys        []string
	PrimaryKeysIdxs    []int

	// DataType is the column's native type as reported by introspection
	// (e.g. varchar(255), int4, mood, enum('a','b')). Empty for
	// computed/expression columns.
	DataType string
	// EnumValues holds the allowed values when DataType is an enumerated
	// type (PG native enum, MySQL enum/set). Empty for every other type;
	// the frontend renders a picker only when this is non-empty.
	EnumValues []string
}

// Result is the buffered output of a query.
//
// Both the local-execution path and the proxified path produce a Result
// (the proxified path drains the wire stream into one).
//
// Page and PageSize are only set by Client.Page; they are zero
// on the full result returned by Execute.
type Result struct {
	Columns        []string
	Rows           [][]any
	RowCount       int
	AffectedRows   int64
	DurationMs     int64
	ColumnEditMeta []ColumnEditMeta
	Errors         []string
	// 1-based character offset, when the driver reports a syntax position.
	ErrorPosition *int
	Page          int
	PageSize int
}
