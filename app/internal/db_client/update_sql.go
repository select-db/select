package db_client

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

// TableEditInput is the payload for a single cell edit (mirrors frontend TableEdit).
type TableEditInput struct {
	DatabaseID       string                 `json:"databaseId"`
	Schema           string                 `json:"schema"`
	Table            string                 `json:"table"`
	Column           string                 `json:"column"`
	Value            string                 `json:"value"`
	RowIndex         int                    `json:"rowIndex"`
	ColumnIndex      int                    `json:"columnIndex"`
	PrimaryKeyValues map[string]interface{} `json:"primaryKeyValues"`
}

// GenerateUpdateSQLParams is the input for GenerateUpdateSQL.
type GenerateUpdateSQLParams struct {
	DatabaseID string           `json:"databaseId"`
	Edits      []TableEditInput `json:"edits"`
}

// GenerateUpdateSQLResult is the output of GenerateUpdateSQL.
type GenerateUpdateSQLResult struct {
	SQL string `json:"sql"`
}

// GenerateUpdateSQL builds a single SQL string of UPDATE statements (one per row),
// ordered by table then by row, using the file's dialect and schema metadata for quoting and literal formatting.
func (dbc *DbClient) GenerateUpdateSQL(params GenerateUpdateSQLParams) (GenerateUpdateSQLResult, error) {
	var out GenerateUpdateSQLResult
	if len(params.Edits) == 0 {
		return out, nil
	}

	dbInstance := dbc.Graph.GetDBInstanceNodeByID(params.DatabaseID)
	if dbInstance == nil {
		return out, fmt.Errorf("database not found")
	}

	dialect := engine.GetDialect(dbInstance.DBType)
	if dialect == nil {
		return out, fmt.Errorf("unsupported DB type for update SQL: %s", dbInstance.DBType)
	}

	metadata, err := dbc.getCachedMetadata(dbInstance, false)
	if err != nil {
		return out, fmt.Errorf("failed to get schema metadata: %w", err)
	}

	sql, err := buildUpdateSQL(params.Edits, dialect, metadata)
	if err != nil {
		return out, err
	}
	out.SQL = sql
	return out, nil
}

// rowGroupKey returns a stable key for grouping edits by (schema, table, row).
// Row identity is given by primaryKeyValues; we sort PK keys for deterministic key.
func rowGroupKey(schema, table string, pk map[string]interface{}) string {
	var parts []string
	for k := range pk {
		parts = append(parts, k)
	}
	sort.Strings(parts)
	var b strings.Builder
	for _, k := range parts {
		b.WriteString(k)
		b.WriteString("=")
		fmt.Fprint(&b, pk[k])
		b.WriteString("\t")
	}
	return schema + "\t" + table + "\t" + b.String()
}

// Group edits by (schema, table, primaryKeyValues)
type group struct {
	schema   string
	table    string
	rowIndex int
	pk       map[string]interface{}
	edits    []TableEditInput
}

func buildUpdateSQL(edits []TableEditInput, dialect core.SQLDialect, metadata *core.Metadata) (string, error) {
	reserved := dialect.GetReservedKeywords()
	_, tableMap := buildSchemaTableMaps(metadata)

	keyToGroup := make(map[string]*group)

	for i := range edits {
		e := &edits[i]
		k := rowGroupKey(e.Schema, e.Table, e.PrimaryKeyValues)
		if g, ok := keyToGroup[k]; ok {
			g.edits = append(g.edits, *e)
			// Keep minimum rowIndex for sorting
			if e.RowIndex < g.rowIndex {
				g.rowIndex = e.RowIndex
			}
		} else {
			keyToGroup[k] = &group{
				schema:   e.Schema,
				table:    e.Table,
				rowIndex: e.RowIndex,
				pk:       e.PrimaryKeyValues,
				edits:    []TableEditInput{*e},
			}
		}
	}

	// Collect groups and sort by table then row
	var groups []*group
	for _, g := range keyToGroup {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].table != groups[j].table {
			return groups[i].table < groups[j].table
		}
		return groups[i].rowIndex < groups[j].rowIndex
	})

	var stmts []string
	for _, g := range groups {
		stmt, err := buildOneUpdate(g, dialect, reserved, tableMap)
		if err != nil {
			return "", err
		}
		stmts = append(stmts, stmt)
	}
	if len(stmts) == 0 {
		return "", nil
	}
	return strings.Join(stmts, "\n\n") + "\n", nil
}

func buildOneUpdate(
	g *group,
	dialect core.SQLDialect,
	reserved map[string]bool,
	tableMap map[string]*core.Table,
) (string, error) {
	quotedSchema := dialect.QuoteIdentifierIfNeeded(g.schema, false, reserved)
	quotedTable := dialect.QuoteIdentifierIfNeeded(g.table, false, reserved)
	tableQual := quotedSchema + "." + quotedTable

	var setParts []string
	for _, e := range g.edits {
		quotedCol := dialect.QuoteIdentifierIfNeeded(e.Column, false, reserved)
		colType := getColumnType(tableMap, g.schema, g.table, e.Column)
		lit, err := formatLiteral(e.Value, colType)
		if err != nil {
			return "", err
		}
		setParts = append(setParts, "  "+quotedCol+" = "+lit)
	}

	var whereParts []string
	// Sort PK keys for deterministic WHERE order
	var pkKeys []string
	for k := range g.pk {
		pkKeys = append(pkKeys, k)
	}
	sort.Strings(pkKeys)
	for _, k := range pkKeys {
		quotedCol := dialect.QuoteIdentifierIfNeeded(k, false, reserved)
		colType := getColumnType(tableMap, g.schema, g.table, k)
		lit, err := formatLiteral(fmt.Sprint(g.pk[k]), colType)
		if err != nil {
			return "", err
		}
		whereParts = append(whereParts, "  "+quotedCol+" = "+lit)
	}

	return "UPDATE " + tableQual + "\nSET\n" + strings.Join(setParts, ",\n") + "\nWHERE\n" + strings.Join(whereParts, "\n  AND ") + ";", nil
}

func getColumnType(tableMap map[string]*core.Table, schema, table, column string) string {
	t := findTable(tableMap, make(map[string]*core.Table), schema, table)
	if t == nil {
		return ""
	}
	for _, c := range t.Columns {
		if strings.EqualFold(c.Name, column) {
			return c.Type
		}
	}
	return ""
}

// formatLiteral produces a SQL literal for the value. Backend is responsible for literal formatting
// using dialect and schema metadata: NULL, boolean, numeric unquoted, strings quoted and escaped.
func formatLiteral(value string, columnType string) (string, error) {
	val := strings.TrimSpace(value)
	if strings.EqualFold(val, "NULL") {
		return "NULL", nil
	}
	// If column type is boolean, output TRUE/FALSE
	if isBooleanType(columnType) {
		if b, ok := parseBooleanLiteral(val); ok {
			if b {
				return "TRUE", nil
			}
			return "FALSE", nil
		}
	}
	// If column type looks numeric, try to output unquoted number
	if isNumericType(columnType) {
		if isNumericLiteral(val) {
			return val, nil
		}
	}
	// String: quote and escape single quotes (double them)
	return "'" + strings.ReplaceAll(val, "'", "''") + "'", nil
}

func isNumericType(t string) bool {
	u := strings.ToUpper(t)
	switch {
	case strings.Contains(u, "INT"),
		strings.Contains(u, "SERIAL"),
		strings.Contains(u, "BIGINT"),
		strings.Contains(u, "SMALLINT"),
		strings.Contains(u, "NUMERIC"),
		strings.Contains(u, "DECIMAL"),
		strings.Contains(u, "REAL"),
		strings.Contains(u, "FLOAT"),
		strings.Contains(u, "DOUBLE"):
		return true
	}
	return false
}

func isNumericLiteral(s string) bool {
	if s == "" {
		return false
	}
	dotCount := 0
	for i, r := range s {
		if r == '.' {
			dotCount++
			if dotCount > 1 {
				return false
			}
		} else if r == '-' || r == '+' {
			// Sign only allowed at the start
			if i != 0 {
				return false
			}
		} else if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isBooleanType(t string) bool {
	u := strings.ToUpper(t)
	return u == "BOOLEAN" || u == "BOOL"
}

// parseBooleanLiteral attempts to parse a string as a boolean value.
// Returns the boolean value and true if successful, false otherwise.
func parseBooleanLiteral(s string) (bool, bool) {
	lower := strings.ToLower(s)
	switch lower {
	case "true", "t", "yes", "y", "on", "1":
		return true, true
	case "false", "f", "no", "n", "off", "0":
		return false, true
	}
	return false, false
}
