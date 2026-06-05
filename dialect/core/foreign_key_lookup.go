package core

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
