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
		Matches: func(column, pattern, escape string) string {
			// SQLite's default LIKE is case-insensitive for ASCII but case
			// sensitive for non-ASCII. Wrapping both sides in LOWER() makes
			// the match symmetric for ASCII; non-ASCII case folding stays a
			// known limitation of SQLite's stock LIKE implementation.
			return fmt.Sprintf("LOWER(CAST(%s AS TEXT)) LIKE LOWER(%s) ESCAPE '%s'", column, pattern, escape)
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
