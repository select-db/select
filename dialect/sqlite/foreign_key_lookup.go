package sqlite

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
		Matches: func(column, pattern string) string {
			// SQLite's LIKE has no default escape character, so without the
			// ESCAPE clause a % or _ in the search text always wildcards. It is
			// also case-insensitive for ASCII only, which is why both sides are
			// lowered; non-ASCII folding stays a limitation of its stock LIKE.
			return fmt.Sprintf("LOWER(CAST(%s AS TEXT)) LIKE LOWER(%s) ESCAPE '%s'", column, pattern, core.LikeEscape)
		},
		Equals: func(column, literal string) string {
			return fmt.Sprintf("CAST(%s AS TEXT) = %s", column, literal)
		},
	})
}

// fkQuoteIdent uses double quotes, the SQL-standard form SQLite accepts for
// identifiers. Embedded double quotes are doubled.
func fkQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// fkQuoteLiteral produces a SQLite string literal. A single quote is escaped by
// doubling it, and that is the only escape: there is no backslash processing.
func fkQuoteLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
