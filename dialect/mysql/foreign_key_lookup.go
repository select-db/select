package mysql

import (
	"fmt"
	"strings"

	"github.com/selectDb/dialect/core"
)

// fkLikeEscape is the LIKE escape character used by the FK picker query.
//
// We deliberately pick "$" rather than backslash. With the default MySQL
// sql_mode (NO_BACKSLASH_ESCAPES disabled), backslash is processed inside
// string literals before the LIKE pattern matcher sees it, so "\_" in a
// 'single-quoted' literal arrives at LIKE as "_". Using "$" sidesteps the
// double-encoding entirely: it has no meaning in MySQL string literals and is
// not a LIKE wildcard, so it carries meaning only via the ESCAPE clause.
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
			// CAST to CHAR is the MySQL spelling for "to text". Wrapping both
			// sides in LOWER() gives a stable case-insensitive match across
			// every collation (utf8mb4_bin would otherwise be case-sensitive).
			ors[i] = fmt.Sprintf("LOWER(CAST(%s AS CHAR)) LIKE LOWER(%s) ESCAPE '%s'",
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

func fkQualifiedTable(schema, table string) string {
	// MySQL treats the schema name as a database name. An empty schema means
	// "use the currently connected database" and must not be qualified.
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
	return fmt.Sprintf("CAST(%s AS CHAR) = %s", fkQuoteIdent(col), fkQuoteLiteral(val))
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
