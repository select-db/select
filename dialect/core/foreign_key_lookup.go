package core

import (
	"fmt"
	"strings"
)

// ForeignKeyLookupParams describes a SELECT to populate the foreign-key picker
// in the cell editor. The frontend hands one of these to the backend; the
// backend asks the dialect to build the actual SQL string.
//
// Identifier values (Schema, Table, FKColumn, DisplayColumns) are unquoted raw
// names. The dialect is responsible for quoting them safely.
//
// Query is the user's raw search input, including any % and _ characters. The
// dialect is responsible for escaping LIKE wildcards so they match literally,
// and for escaping string-literal characters so user input cannot break out of
// the SQL string. The empty string means "no filter".
//
// CurrentValue is the existing FK value of the row being edited, as a string.
// When non-empty, the dialect must:
//   - include the row whose FK column equals CurrentValue in the result, even
//     when Query would have filtered it out;
//   - sort that row to the very top of the result.
//
// An empty CurrentValue means there is nothing to pin (typically a NULL cell).
//
// Limit > 0 caps the number of rows returned. Zero means no LIMIT clause.
type ForeignKeyLookupParams struct {
	Schema         string
	Table          string
	FKColumn       string
	DisplayColumns []string
	Query          string
	CurrentValue   string
	Limit          int
}

// ForeignKeySQLSyntax is what building the picker's query needs from a dialect:
// how it quotes, and how it compares a column to text. The rest of the query is
// the same everywhere.
//
// QuoteIdent and QuoteLiteral quote unconditionally. Matches is the
// case-insensitive LIKE of an already-quoted column against an already-quoted
// pattern, with escape as the ESCAPE character. Equals compares an
// already-quoted column to an already-quoted literal as text.
type ForeignKeySQLSyntax struct {
	QuoteIdent   func(name string) string
	QuoteLiteral func(value string) string
	Matches      func(column, pattern, escape string) string
	Equals       func(column, literal string) string
}

// LikeEscape is the character the picker's LIKE patterns escape with. Every
// dialect uses it: it means nothing in a string literal and is not a wildcard,
// so it carries meaning only through the ESCAPE clause.
const LikeEscape = "$"

// BuildForeignKeyLookupSQL builds the picker's SELECT. The row p.CurrentValue
// names is kept whatever the search says and sorted to the top, so the cell's
// existing value is always one of the choices.
func BuildForeignKeyLookupSQL(p ForeignKeyLookupParams, s ForeignKeySQLSyntax) string {
	columns := dedupKeepOrder(append([]string{p.FKColumn}, p.DisplayColumns...))
	for i, c := range columns {
		columns[i] = s.QuoteIdent(c)
	}

	from := s.QuoteIdent(p.Table)
	if p.Schema != "" {
		from = s.QuoteIdent(p.Schema) + "." + from
	}

	current := ""
	if p.CurrentValue != "" {
		current = s.Equals(s.QuoteIdent(p.FKColumn), s.QuoteLiteral(p.CurrentValue))
	}

	var where string
	if q := strings.TrimSpace(p.Query); q != "" {
		searched := p.DisplayColumns
		if len(searched) == 0 {
			searched = []string{p.FKColumn}
		}
		pattern := s.QuoteLiteral("%" + EscapeLike(q) + "%")
		ors := make([]string, len(searched))
		for i, c := range searched {
			ors[i] = s.Matches(s.QuoteIdent(c), pattern, LikeEscape)
		}
		where = " WHERE (" + strings.Join(ors, " OR ") + ")"
		if current != "" {
			where += " OR " + current
		}
	}

	var orderBy string
	if current != "" {
		orderBy = " ORDER BY " + current + " DESC"
	}

	var limit string
	if p.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", p.Limit)
	}

	return "SELECT " + strings.Join(columns, ", ") + " FROM " + from + where + orderBy + limit
}

// EscapeLike escapes the LIKE wildcards % and _, and the escape character
// itself, so the search text matches literally.
func EscapeLike(v string) string {
	v = strings.ReplaceAll(v, LikeEscape, LikeEscape+LikeEscape)
	v = strings.ReplaceAll(v, "%", LikeEscape+"%")
	v = strings.ReplaceAll(v, "_", LikeEscape+"_")
	return v
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
