package mysql

import (
	"context"
	"database/sql"
	"strings"

	core "github.com/selectDb/dialect/core"
	parser "github.com/selectDb/dialect/mysql/parser"
)

const mysqlBuiltinSchema = "mysql_builtin"

// mysqlBuiltinTypes is the static catalog of MySQL built-in column types.
// MySQL has no CREATE TYPE, so the catalog is hardcoded rather than fetched.
// Surfaced via GetCatalogSchema for completion, hover, and the Types tree.
// Common aliases (INTEGER, NUMERIC, REAL, BOOL, ...) are listed as distinct
// entries so completion suggests whichever spelling the user starts to type.
var mysqlBuiltinTypes = []core.Type{
	{Name: "TINYINT", Kind: "builtin", Display: "TINYINT", Description: "1-byte signed integer (-128 to 127); unsigned 0 to 255."},
	{Name: "SMALLINT", Kind: "builtin", Display: "SMALLINT", Description: "2-byte signed integer (-32768 to 32767)."},
	{Name: "MEDIUMINT", Kind: "builtin", Display: "MEDIUMINT", Description: "3-byte signed integer (-8388608 to 8388607)."},
	{Name: "INT", Kind: "builtin", Display: "INT", Description: "4-byte signed integer."},
	{Name: "INTEGER", Kind: "builtin", Display: "INTEGER", Description: "Alias for INT."},
	{Name: "BIGINT", Kind: "builtin", Display: "BIGINT", Description: "8-byte signed integer."},
	{Name: "DECIMAL", Kind: "builtin", Display: "DECIMAL", Description: "Exact fixed-point number, DECIMAL(M,D)."},
	{Name: "NUMERIC", Kind: "builtin", Display: "NUMERIC", Description: "Alias for DECIMAL."},
	{Name: "DEC", Kind: "builtin", Display: "DEC", Description: "Alias for DECIMAL."},
	{Name: "FIXED", Kind: "builtin", Display: "FIXED", Description: "Alias for DECIMAL."},
	{Name: "FLOAT", Kind: "builtin", Display: "FLOAT", Description: "Single-precision floating-point number (4 bytes)."},
	{Name: "DOUBLE", Kind: "builtin", Display: "DOUBLE", Description: "Double-precision floating-point number (8 bytes)."},
	{Name: "REAL", Kind: "builtin", Display: "REAL", Description: "Alias for DOUBLE (or FLOAT when REAL_AS_FLOAT is set)."},
	{Name: "BIT", Kind: "builtin", Display: "BIT", Description: "Bit-field, BIT(M) from 1 to 64 bits."},
	{Name: "BOOLEAN", Kind: "builtin", Display: "BOOLEAN", Description: "Synonym for TINYINT(1); 0 = false, non-zero = true."},
	{Name: "BOOL", Kind: "builtin", Display: "BOOL", Description: "Alias for BOOLEAN."},

	{Name: "CHAR", Kind: "builtin", Display: "CHAR", Description: "Fixed-length string up to 255 characters."},
	{Name: "VARCHAR", Kind: "builtin", Display: "VARCHAR", Description: "Variable-length string up to 65535 bytes per row."},
	{Name: "BINARY", Kind: "builtin", Display: "BINARY", Description: "Fixed-length binary string."},
	{Name: "VARBINARY", Kind: "builtin", Display: "VARBINARY", Description: "Variable-length binary string."},
	{Name: "TINYTEXT", Kind: "builtin", Display: "TINYTEXT", Description: "Variable-length text up to 255 bytes."},
	{Name: "TEXT", Kind: "builtin", Display: "TEXT", Description: "Variable-length text up to 65535 bytes."},
	{Name: "MEDIUMTEXT", Kind: "builtin", Display: "MEDIUMTEXT", Description: "Variable-length text up to 16 MB."},
	{Name: "LONGTEXT", Kind: "builtin", Display: "LONGTEXT", Description: "Variable-length text up to 4 GB."},
	{Name: "TINYBLOB", Kind: "builtin", Display: "TINYBLOB", Description: "Variable-length binary data up to 255 bytes."},
	{Name: "BLOB", Kind: "builtin", Display: "BLOB", Description: "Variable-length binary data up to 65535 bytes."},
	{Name: "MEDIUMBLOB", Kind: "builtin", Display: "MEDIUMBLOB", Description: "Variable-length binary data up to 16 MB."},
	{Name: "LONGBLOB", Kind: "builtin", Display: "LONGBLOB", Description: "Variable-length binary data up to 4 GB."},
	{Name: "ENUM", Kind: "builtin", Display: "ENUM", Description: "String object with a value chosen from a list of permitted values declared inline."},
	{Name: "SET", Kind: "builtin", Display: "SET", Description: "String object that can have zero or more values from a list declared inline."},

	{Name: "DATE", Kind: "builtin", Display: "DATE", Description: "Calendar date (YYYY-MM-DD)."},
	{Name: "TIME", Kind: "builtin", Display: "TIME", Description: "Time of day (HH:MM:SS), range -838:59:59 to 838:59:59."},
	{Name: "DATETIME", Kind: "builtin", Display: "DATETIME", Description: "Date and time (YYYY-MM-DD HH:MM:SS)."},
	{Name: "TIMESTAMP", Kind: "builtin", Display: "TIMESTAMP", Description: "UTC date and time, auto-converted to and from the session time zone."},
	{Name: "YEAR", Kind: "builtin", Display: "YEAR", Description: "Year in 4-digit format, range 1901 to 2155 and 0000."},

	{Name: "JSON", Kind: "builtin", Display: "JSON", Description: "Native JSON document storage with validation and path operators."},

	{Name: "GEOMETRY", Kind: "builtin", Display: "GEOMETRY", Description: "Generic spatial value."},
	{Name: "POINT", Kind: "builtin", Display: "POINT", Description: "Spatial point (X, Y)."},
	{Name: "LINESTRING", Kind: "builtin", Display: "LINESTRING", Description: "Spatial line."},
	{Name: "POLYGON", Kind: "builtin", Display: "POLYGON", Description: "Spatial polygon."},
	{Name: "MULTIPOINT", Kind: "builtin", Display: "MULTIPOINT", Description: "Collection of points."},
	{Name: "MULTILINESTRING", Kind: "builtin", Display: "MULTILINESTRING", Description: "Collection of lines."},
	{Name: "MULTIPOLYGON", Kind: "builtin", Display: "MULTIPOLYGON", Description: "Collection of polygons."},
	{Name: "GEOMETRYCOLLECTION", Kind: "builtin", Display: "GEOMETRYCOLLECTION", Description: "Collection of mixed spatial values."},
}

// mysqlBuiltinFunctions wraps the parser's flat name list as core.Function
// entries so they surface alongside built-in types in the catalog schema.
// Args/Result/Description are left empty: the parser only carries names, and
// synthesising signatures for hundreds of functions belongs in a dedicated
// catalog rather than here.
var mysqlBuiltinFunctions = buildBuiltinFunctions()

func buildBuiltinFunctions() []core.Function {
	names := parser.GetBuiltinFunctions()
	out := make([]core.Function, 0, len(names))
	for i, name := range names {
		out = append(out, core.Function{
			Schema: mysqlBuiltinSchema,
			Name:   name,
			Kind:   "function",
			OID:    int64(i + 1),
		})
	}
	return out
}

// enrichBuiltinFunctionsFromHelp fills Description and best-effort Args on the
// static builtin list by joining against mysql.help_topic. The WHERE list is
// generated from the parser's name slice so we never overfetch. On any error
// (table missing, no permission, empty help corpus), returns base unchanged.
func enrichBuiltinFunctionsFromHelp(ctx context.Context, db *sql.DB, base []core.Function) []core.Function {
	if db == nil || len(base) == 0 {
		return base
	}

	placeholders := make([]string, len(base))
	args := make([]any, len(base))
	for i, f := range base {
		placeholders[i] = "?"
		args[i] = f.Name
	}

	query := "SELECT name, description FROM mysql.help_topic WHERE name IN (" +
		strings.Join(placeholders, ",") + ")"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return base
	}
	defer func() { _ = rows.Close() }()

	help := make(map[string]string, len(base))
	for rows.Next() {
		var name, desc string
		if err := rows.Scan(&name, &desc); err != nil {
			return base
		}
		help[strings.ToUpper(name)] = desc
	}
	if rows.Err() != nil || len(help) == 0 {
		return base
	}

	out := make([]core.Function, len(base))
	copy(out, base)
	for i := range out {
		desc, ok := help[strings.ToUpper(out[i].Name)]
		if !ok {
			continue
		}
		out[i].Description = strings.TrimSpace(desc)
		out[i].Args = extractHelpArgs(out[i].Name, desc)
	}
	return out
}

// extractHelpArgs scans a help_topic description for the first NAME(...)
// signature and returns the parenthesised argument list. Returns "" when the
// function has no parenthesised form (e.g. CASE) or the description lacks one.
func extractHelpArgs(name, desc string) string {
	needle := strings.ToUpper(name) + "("
	upper := strings.ToUpper(desc)
	idx := strings.Index(upper, needle)
	if idx < 0 {
		return ""
	}
	start := idx + len(needle)
	end := strings.Index(desc[start:], ")")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(desc[start : start+end])
}
