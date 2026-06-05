package sqlite

import (
	"fmt"
	"strings"

	"github.com/selectDb/dialect/core"
)

// fkLikeEscape is the LIKE escape character used by the FK picker query.
//
// SQLite's LIKE doesn't have a default escape character, so without ESCAPE
// the % and _ in a pattern always wildcard. We pick "$" because it has no
// meaning in SQLite string literals and isn't a LIKE wildcard, so it carries
// meaning only via the ESCAPE clause we attach.
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
			// SQLite's default LIKE is case-insensitive for ASCII but case
			// sensitive for non-ASCII. Wrapping both sides in LOWER() makes
			// the match symmetric for ASCII; non-ASCII case folding stays a
			// known limitation of SQLite's stock LIKE implementation.
			ors[i] = fmt.Sprintf("LOWER(CAST(%s AS TEXT)) LIKE LOWER(%s) ESCAPE '%s'",
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

// fkQuoteIdent uses double quotes, the SQL-standard form SQLite accepts for
// identifiers. Embedded double quotes are doubled.
func fkQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// fkQuoteLiteral produces a SQLite string literal. SQLite literals only ever
// escape a single quote (as ''); there is no backslash processing.
func fkQuoteLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func fkQualifiedTable(schema, table string) string {
	// In SQLite a "schema" is really an attached database name (main, temp,
	// or whatever the user attached with ATTACH DATABASE). When empty we
	// leave it off and rely on the default search order.
	if schema == "" {
		return fkQuoteIdent(table)
	}
	return fkQuoteIdent(schema) + "." + fkQuoteIdent(table)
}

func fkEscapeLike(v string) string {
	v = strings.ReplaceAll(v, fkLikeEscape, fkLikeEscape+fkLikeEscape)
	v = strings.ReplaceAll(v, "%", fkLikeEscape+"%")
	v = strings.ReplaceAll(v, "_", fkLikeEscape+"_")
	return v
}

func fkCurrentValuePredicate(col, val string) string {
	return fmt.Sprintf("CAST(%s AS TEXT) = %s", fkQuoteIdent(col), fkQuoteLiteral(val))
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
