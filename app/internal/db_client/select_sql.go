package db_client

import (
	"fmt"
	"strings"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

// DefaultSelectPreviewLimit caps the row count of the SELECT generated for the
// "View data" action. Previewing a table must never turn into a full table scan
// streamed back to the UI, and the user can always widen the LIMIT afterwards.
const DefaultSelectPreviewLimit = 100

// GenerateSelectSQLParams is the payload for GenerateSelectSQL. Like the FK
// lookup it is deliberately structured (no SQL strings on the wire) so quoting
// stays dialect-aware and stays out of the frontend.
type GenerateSelectSQLParams struct {
	DatabaseID string `json:"databaseId"`
	Schema     string `json:"schema"`
	Table      string `json:"table"`
	// Limit is the row cap of the generated statement. Values <= 0 fall back to
	// DefaultSelectPreviewLimit.
	Limit int `json:"limit"`
}

// GenerateSelectSQLResult is the output of GenerateSelectSQL.
type GenerateSelectSQLResult struct {
	SQL string `json:"sql"`
}

// GenerateSelectSQL builds a preview SELECT for a single relation, using the
// database instance's dialect for identifier quoting.
func (dbc *DbClient) GenerateSelectSQL(params GenerateSelectSQLParams) (GenerateSelectSQLResult, error) {
	var out GenerateSelectSQLResult

	dbInstance := dbc.Graph.GetDBInstanceNodeByID(params.DatabaseID)
	if dbInstance == nil {
		return out, fmt.Errorf("database not found")
	}

	dialect := engine.GetDialect(dbInstance.DBType)
	if dialect == nil {
		return out, fmt.Errorf("unsupported DB type for select SQL: %s", dbInstance.DBType)
	}

	sql, err := buildSelectSQL(params.Schema, params.Table, params.Limit, dialect)
	if err != nil {
		return out, err
	}
	out.SQL = sql
	return out, nil
}

// buildSelectSQL renders "SELECT * FROM <qualified table> LIMIT <n>;".
// An empty schema is left out of the qualification: MySQL reads it as "the
// connected database" and SQLite as "main", so qualifying with "" would be wrong.
func buildSelectSQL(schema, table string, limit int, dialect core.SQLDialect) (string, error) {
	tableName := strings.TrimSpace(table)
	if tableName == "" {
		return "", fmt.Errorf("table name is required")
	}

	reserved := dialect.GetReservedKeywords()
	qualified := dialect.QuoteIdentifierIfNeeded(tableName, false, reserved)

	if schemaName := strings.TrimSpace(schema); schemaName != "" {
		qualified = dialect.QuoteIdentifierIfNeeded(schemaName, false, reserved) + "." + qualified
	}

	if limit <= 0 {
		limit = DefaultSelectPreviewLimit
	}

	return fmt.Sprintf("SELECT *\nFROM %s\nLIMIT %d;\n", qualified, limit), nil
}
