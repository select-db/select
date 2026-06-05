package core

import (
	"context"
	"database/sql"
	"encoding/json"

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

// SQLDialect defines the interface that all database dialects must implement
type SQLDialect interface {
	// Name returns the dialect name (e.g., "postgresql", "mysql", "sqlite")
	Name() string

	// OpenDB opens a database connection using the dialect's driver.
	// It returns a *sql.DB connection that can be used for querying the database.
	// The caller is responsible for closing the connection when done.
	OpenDB(dsn string) (*sql.DB, error)

	// GetSchemas returns all user schemas in the database.
	// For PostgreSQL, this excludes system schemas (pg_*, information_schema).
	// For SQLite, this returns ["main"] as SQLite doesn't have real schemas.
	// For MySQL, this returns an empty slice as MySQL uses databases instead of schemas.
	GetSchemas(ctx context.Context, db *sql.DB) ([]string, error)

	// GetCurrentSchema returns the current active schema for the session.
	// For PostgreSQL, this queries current_schema() or the first schema in search_path.
	// For SQLite, this returns "main".
	// For MySQL, this returns an empty string (MySQL uses databases, not schemas).
	GetCurrentSchema(ctx context.Context, db *sql.DB) (string, error)

	// DefaultSchemaName is the conventional default schema for this dialect,
	// used as a fallback when GetCurrentSchema reports nothing. Returns ""
	// for dialects without a meaningful default (e.g. MySQL).
	DefaultSchemaName() string

	// GetTables returns all tables in a specific schema.
	GetTables(ctx context.Context, db *sql.DB, schema string) ([]Table, error)

	// GetViews returns all views in a specific schema.
	GetViews(ctx context.Context, db *sql.DB, schema string) ([]Table, error)

	// GetIndexes returns all indexes in a specific schema.
	GetIndexes(ctx context.Context, db *sql.DB, schema string) ([]IndexInfo, error)

	// GetTriggers returns all triggers in a specific schema.
	GetTriggers(ctx context.Context, db *sql.DB, schema string) ([]TriggerInfo, error)

	// GetStats returns statistics for tables and indexes in a specific schema.
	GetStats(ctx context.Context, db *sql.DB, schema string) (TableStats, error)

	// GetTypes returns types for the given user schema.
	GetTypes(ctx context.Context, db *sql.DB, schema string) ([]Type, error)
	// GetFunctions returns callable routines for the given user schema.
	GetFunctions(ctx context.Context, db *sql.DB, schema string) ([]Function, error)
	// GetCatalogSchema returns the system/built-in catalog as a fully-populated Schema,
	// or nil if the dialect does not have one. For PostgreSQL this is pg_catalog
	// (tables, views, types, functions). For SQLite this is a synthetic schema with
	// built-in types and functions.
	GetCatalogSchema(ctx context.Context, db *sql.DB) (*Schema, error)
	// GetSettings returns runtime parameters (MySQL system variables, PG GUCs,
	// SQLite pragmas). Stashed on the catalog schema.
	GetSettings(ctx context.Context, db *sql.DB) ([]Setting, error)

	// ANTLR grammar (used by inspect system only)
	CreateLexer(input string) antlr.Lexer
	CreateParser(stream antlr.TokenStream) antlr.Parser
	InferColumnsFromSubquery(parser antlr.Parser, meta Metadata, defaultSchema string) []Column

	// Syntax features
	GetReservedKeywords() map[string]bool
	GetBuiltinFunctions() []string
	GetDefaultKeywords() []string
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

	// Inspect parses sql against meta and returns one InspectStatement per top-level statement.
	// Returns nil for dialects that do not yet support inspection (e.g. MySQL).
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
