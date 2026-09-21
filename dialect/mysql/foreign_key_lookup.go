package mysql

import (
	"fmt"
	"strings"

	"github.com/selectDb/dialect/core"
)

// BuildForeignKeyLookupSQL implements core.SQLDialect.
func (d *Dialect) BuildForeignKeyLookupSQL(p core.ForeignKeyLookupParams) string {
	return core.BuildForeignKeyLookupSQL(p, core.ForeignKeySQLSyntax{
		QuoteIdent:   fkQuoteIdent,
		QuoteLiteral: fkQuoteLiteral,
		Matches: func(column, pattern, escape string) string {
			// CAST to CHAR is the MySQL spelling for "to text". Wrapping both
			// sides in LOWER() gives a stable case-insensitive match across
			// every collation (utf8mb4_bin would otherwise be case-sensitive).
			return fmt.Sprintf("LOWER(CAST(%s AS CHAR)) LIKE LOWER(%s) ESCAPE '%s'", column, pattern, escape)
		},
		Equals: func(column, literal string) string {
			return fmt.Sprintf("CAST(%s AS CHAR) = %s", column, literal)
		},
	})
}

// fkQuoteIdent uses backticks, which work regardless of ANSI_QUOTES sql_mode.
// Embedded backticks are doubled. The FK picker doesn't have the reserved
// keyword map available, and unconditional quoting is safe for any identifier.
func fkQuoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// fkQuoteLiteral produces a MySQL string literal that is safe under both the
// default sql_mode (backslash escapes enabled) and NO_BACKSLASH_ESCAPES. We
// double both single quotes and backslashes, so the literal we emit is
// identical in either mode and contains no characters that need the LIKE
// escape character to survive.
func fkQuoteLiteral(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, "'", "''")
	return "'" + v + "'"
}
