package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"

	antlr "github.com/antlr4-go/antlr/v4"
)

// Analyzer is the interface for the Python subprocess that handles dialect-agnostic
// SQL analysis (reference collection, completion context, linting). Implemented by
// lint.Analyzer.
type Analyzer interface {
	Call(req map[string]any) (json.RawMessage, error)
}

// Feature represents a SQL feature that may or may not be supported by a dialect
type Feature int

const (
	FeatureCTE Feature = iota
	FeatureMaterializedViews
	FeatureRecursiveCTE
	FeatureWindowFunctions
	FeatureForeignTables
	FeatureSchemas
)

// DialFunc dials address for a driver; it may refuse the address.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// SQLDialect defines the interface that all database dialects must implement
type SQLDialect interface {
	// Name returns the dialect name (e.g., "postgresql", "mysql", "sqlite")
	Name() string

	// OpenDB opens a database connection using the dialect's driver.
	// It returns a *sql.DB connection that can be used for querying the database.
	// The caller is responsible for closing the connection when done.
	OpenDB(dsn string) (*sql.DB, error)

	// OpenGuardedDB is OpenDB with every connection dialed through dial. A
	// dialect that opens no network connection refuses.
	OpenGuardedDB(dsn string, dial DialFunc) (*sql.DB, error)

	// DSNHost returns the host and port the driver dials. A dialect with no
	// network target, or a DSN that does not parse, returns an error.
	DSNHost(dsn string) (host string, port int, err error)
	// DSNWithHost returns dsn pointed at host:port.
	DSNWithHost(dsn, host string, port int) (string, error)
	// DSNPassword returns the password in dsn, or "" when it holds none.
	DSNPassword(dsn string) string
	// DSNWithPassword returns dsn holding password.
	DSNWithPassword(dsn, password string) string
	// DSNWithoutPassword returns dsn with no password.
	DSNWithoutPassword(dsn string) string

	// ReadSchema reads every schema, the built-in catalog, the settings and the
	// current schema in as few round trips as the database allows, the same few
	// whatever its size.
	ReadSchema(ctx context.Context, db *sql.DB) (*Metadata, error)

	// ANTLR grammar (used by inspect system only)
	CreateLexer(input string) antlr.Lexer
	CreateParser(stream antlr.TokenStream) antlr.Parser
	InferColumnsFromSubquery(parser antlr.Parser, meta Metadata, defaultSchema string) []Column

	// Syntax features
	GetReservedKeywords() map[string]bool
	GetBuiltinFunctions() []string
	GetDefaultKeywords() []string
	// KeywordsOutsideGroup names, per completion group, the words this dialect
	// has in some other role but not in that one: SQLite writes SET in an
	// UPDATE and has no SET statement.
	KeywordsOutsideGroup() map[string][]string
	SupportsFeature(feature Feature) bool

	// Identifier handling
	NormalizeIdentifier(raw string) string
	QuoteIdentifierIfNeeded(name string, caretQuoted bool, reserved map[string]bool) string
	IsValidUnquotedIdentifier(name string) bool

	// SetAnalyzer sets the Python analyzer for dialect-agnostic SQL analysis.
	// When set, Complete() uses Python for reference collection and context parsing.
	SetAnalyzer(a Analyzer)

	// Complete provides SQL code completion suggestions for a given SQL string and caret position
	Complete(ctx context.Context, sql string, caretLine, caretOffset int, meta Metadata) ([]Candidate, error)

	// Inspect parses sql against meta and returns one InspectStatement per
	// top-level statement, and UnknownStatement for one it cannot classify.
	// Permission checks read a missing statement as nothing to check, so an
	// implementation returns nil only for blank sql.
	Inspect(meta Metadata, sql string) []InspectStatement

	// Returns operators valid for a given column type
	GetOperatorsForType(columnType string) []OperatorInfo

	// DumpSchema attempts an authoritative schema dump using the native CLI tool for
	// this dialect. dsn is the effective connection string (already tunnel-rewritten
	// for SSH connections). Returns (sql, true) on success, ("", false) when the tool
	// is not installed or the dump fails.
	DumpSchema(dsn string) (string, bool)

	// DumpSchemaWarning returns a SQL comment block to prepend to schema.sql when
	// DumpSchema is unavailable and the file was reconstructed from catalog queries.
	// Returns an empty string for dialects where the fallback is already authoritative.
	DumpSchemaWarning() string

	// BuildForeignKeyLookupSQL builds a SELECT that powers the foreign-key
	// picker in the cell editor. Implementations must:
	//   - quote every identifier in params.Schema/Table/FKColumn/DisplayColumns
	//   - escape params.Query so LIKE wildcards match literally and the user
	//     input cannot break out of the SQL string
	//   - cast columns to text before comparing them, so FK columns of any
	//     non-string type still work
	//   - if params.CurrentValue is non-empty, include the row whose FK column
	//     equals it in the result set and sort that row to the very top
	//   - return a single statement without a trailing semicolon
	BuildForeignKeyLookupSQL(params ForeignKeyLookupParams) string
}

type OperatorInfo struct {
	Text        string // Display text (e.g., "LIKE", "BETWEEN")
	InsertText  string // Snippet template (e.g., "LIKE '$0'", "BETWEEN $1 AND $0")
	Description string // Short description of what the operator does
}

// RelationRefListener interface for dialect-agnostic table reference collection
type RelationRefListener interface {
	GetReferences() []RelationRef
	GetVirtualTables() []RelationRef
	SetReferences(refs []RelationRef)
	SetVirtualTables(vtabs []RelationRef)
	GetDefaultSchema() string
	GetMeta() Metadata
}

// Registry for dialect implementations
var registry = make(map[string]SQLDialect)

// Register registers a dialect implementation
func Register(name string, dialect SQLDialect) {
	registry[name] = dialect
}

// Get retrieves a registered dialect
func Get(name string) (SQLDialect, error) {
	d, ok := registry[name]
	if !ok {
		return nil, &ErrDialectNotFound{Name: name}
	}
	return d, nil
}

// ErrDialectNotFound is returned when a dialect is not registered
type ErrDialectNotFound struct {
	Name string
}

func (e *ErrDialectNotFound) Error() string {
	return "dialect not found: " + e.Name
}
