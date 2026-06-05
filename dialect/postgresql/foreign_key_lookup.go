package postgresql

import (
	"fmt"
	"strings"

	"github.com/selectDb/dialect/core"
)

// fkLikeEscape is the LIKE/ILIKE escape character used by the FK picker query.
//
// We deliberately pick "$" rather than backslash:
//   - With standard_conforming_strings=on (PostgreSQL >= 9.1 default) the
//     backslash is a literal character in 'single-quoted' literals, so using
//     either would work. But to keep the produced SQL portable across
//     standard_conforming_strings settings, "$" has no quoting quirks at all.
//   - It is not a wildcard for LIKE, so it only carries meaning via ESCAPE.
const fkLikeEscape = "$"

// BuildForeignKeyLookupSQL implements core.SQLDialect.
func (d *Dialect) BuildForeignKeyLookupSQL(p core.ForeignKeyLookupParams) string {
	cols := dedupKeepOrder(append([]string{p.FKColumn}, p.DisplayColumns...))
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = fkQuoteIdent(c)
	}

	from := fkQualifiedTable(p.Schema, p.Table)

	var where string
	if q := strings.TrimSpace(p.Query); q != "" {
		searchCols := p.DisplayColumns
		if len(searchCols) == 0 {
			searchCols = []string{p.FKColumn}
		}
		pattern := fkQuoteLiteral("%" + fkEscapeLike(q) + "%")
		ors := make([]string, len(searchCols))
		for i, c := range searchCols {
			ors[i] = fmt.Sprintf("%s::text ILIKE %s ESCAPE '%s'",
				fkQuoteIdent(c), pattern, fkLikeEscape)
		}
		where = " WHERE (" + strings.Join(ors, " OR ") + ")"
		if p.CurrentValue != "" {
			where += " OR " + fkCurrentValuePredicate(p.FKColumn, p.CurrentValue)
		}
	}

	var orderBy string
	if p.CurrentValue != "" {
		orderBy = " ORDER BY " + fkCurrentValuePredicate(p.FKColumn, p.CurrentValue) + " DESC"
	}

	var limit string
	if p.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", p.Limit)
	}

	return "SELECT " + strings.Join(quotedCols, ", ") + " FROM " + from + where + orderBy + limit
}

// fkQuoteIdent always quotes; the FK picker doesn't have access to the reserved
// keyword set used by QuoteIdentifierIfNeeded, and unconditional quoting is
// safe for every PostgreSQL identifier we might receive from introspection.
func fkQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// fkQuoteLiteral produces a SQL string literal valid under both
// standard_conforming_strings=on (only '' escapes ') and off (no other escapes
// needed, since we deliberately don't use backslash escape sequences here).
func fkQuoteLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func fkQualifiedTable(schema, table string) string {
	if schema == "" {
		return fkQuoteIdent(table)
	}
	return fkQuoteIdent(schema) + "." + fkQuoteIdent(table)
}

// fkEscapeLike escapes the LIKE/ILIKE wildcards % and _, plus the escape
// character itself, so they match literally inside the search pattern.
func fkEscapeLike(v string) string {
	v = strings.ReplaceAll(v, fkLikeEscape, fkLikeEscape+fkLikeEscape)
	v = strings.ReplaceAll(v, "%", fkLikeEscape+"%")
	v = strings.ReplaceAll(v, "_", fkLikeEscape+"_")
	return v
}

// fkCurrentValuePredicate compares the FK column to the current value as text,
// which lets the picker work uniformly for non-string FK types (integer, uuid,
// etc.) since the cell value arrives from the frontend as a string.
func fkCurrentValuePredicate(col, val string) string {
	return fmt.Sprintf("%s::text = %s", fkQuoteIdent(col), fkQuoteLiteral(val))
}

func dedupKeepOrder(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
