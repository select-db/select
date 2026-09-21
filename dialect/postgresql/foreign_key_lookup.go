package postgresql

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
			return fmt.Sprintf("%s::text ILIKE %s ESCAPE '%s'", column, pattern, escape)
		},
		Equals: func(column, literal string) string {
			// Comparing as text lets the picker work for a non-string key
			// (integer, uuid), since the cell value arrives as a string.
			return fmt.Sprintf("%s::text = %s", column, literal)
		},
	})
}

// fkQuoteIdent always quotes; the FK picker doesn't have access to the reserved
// keyword set used by QuoteIdentifierIfNeeded, and unconditional quoting is
// safe for every PostgreSQL identifier we might receive from introspection.
func fkQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// fkQuoteLiteral produces a SQL string literal valid under both
// standard_conforming_strings=on, where doubling is the only escape, and off
// (no other escape is needed, since we do not use backslash sequences here).
func fkQuoteLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
