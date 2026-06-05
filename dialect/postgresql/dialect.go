package postgresql

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	core "github.com/selectDb/dialect/core"

	pg "github.com/selectDb/dialect/postgresql/parser"

	antlr "github.com/antlr4-go/antlr/v4"
	_ "github.com/lib/pq"
)

// Register the PostgreSQL dialect on package init
func init() {
	core.Register("postgresql", NewDialect())
}

// Ensure Dialect implements dialect.SQLDialect interface at compile time
var _ core.SQLDialect = (*Dialect)(nil)

// Dialect implements dialect.SQLDialect for PostgreSQL
type Dialect struct {
	reservedKeywords     map[string]bool
	builtinFunctions     []string
	defaultKeywords      []string
	quotedTokenTypes     map[int]bool
	identifierTokenTypes map[int]bool
	joinKeywords         []int
	endOfClauseKeywords  []int
	analyzer             core.Analyzer
}

// SetAnalyzer sets the Python analyzer for dialect-agnostic reference collection
// and completion context detection. When set, Complete() uses the Python path
// instead of ANTLR token scanning.
func (d *Dialect) SetAnalyzer(a core.Analyzer) {
	d.analyzer = a
}

// NewDialect creates a new PostgreSQL dialect
func NewDialect() *Dialect {
	return &Dialect{
		reservedKeywords:     defaultReserved(),
		builtinFunctions:     pg.GetBuiltinFunctions(),
		defaultKeywords:      defaultKeywords(),
		quotedTokenTypes:     makeQuotedTokenTypes(),
		identifierTokenTypes: makeIdentifierTokenTypes(),
		joinKeywords: []int{
			pg.PostgreSQLLexerFROM, pg.PostgreSQLLexerJOIN,
			pg.PostgreSQLLexerLEFT, pg.PostgreSQLLexerRIGHT,
			pg.PostgreSQLLexerFULL, pg.PostgreSQLLexerCROSS,
		},
		endOfClauseKeywords: []int{
			pg.PostgreSQLLexerFROM, pg.PostgreSQLLexerORDER, pg.PostgreSQLLexerWHERE,
		},
	}
}

func (d *Dialect) Name() string {
	return "postgresql"
}

func (d *Dialect) OpenDB(dsn string) (*sql.DB, error) {
	return openPostgresDBWithFallback(dsn)
}

func openPostgresDBWithFallback(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func (d *Dialect) CreateLexer(input string) antlr.Lexer {
	return pg.NewPostgreSQLLexer(antlr.NewInputStream(input))
}

func (d *Dialect) CreateParser(stream antlr.TokenStream) antlr.Parser {
	parser := pg.NewPostgreSQLParser(stream)
	parser.RemoveErrorListeners()
	return parser
}



func (d *Dialect) GetReservedKeywords() map[string]bool {
	return d.reservedKeywords
}

func (d *Dialect) GetBuiltinFunctions() []string {
	return d.builtinFunctions
}

func (d *Dialect) GetDefaultKeywords() []string {
	return d.defaultKeywords
}

func (d *Dialect) SupportsFeature(feature core.Feature) bool {
	switch feature {
	case core.FeatureCTE, core.FeatureMaterializedViews,
		core.FeatureRecursiveCTE, core.FeatureWindowFunctions,
		core.FeatureForeignTables, core.FeatureSchemas:
		return true
	default:
		return false
	}
}

func (d *Dialect) NormalizeIdentifier(raw string) string {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return normalizePostgreSQLQuotedIdentifier(raw)
	}
	return unquote(raw)
}

func (d *Dialect) QuoteIdentifierIfNeeded(name string, caretQuoted bool, reserved map[string]bool) string {
	if caretQuoted {
		return name
	}
	if strings.ToLower(name) != name {
		return fmt.Sprintf("\"%s\"", escapeDoubleQuotes(name))
	}
	if reserved[strings.ToUpper(name)] {
		return fmt.Sprintf("\"%s\"", escapeDoubleQuotes(name))
	}
	if !d.IsValidUnquotedIdentifier(name) {
		return fmt.Sprintf("\"%s\"", escapeDoubleQuotes(name))
	}
	return name
}

func (d *Dialect) IsValidUnquotedIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := rune(s[0])
	if !unicode.IsLetter(first) && first != '_' {
		return false
	}
	for _, ch := range s[1:] {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			return false
		}
	}
	return true
}





// Syntax token queries for context-aware parsing
























// GetOperatorsForType returns operators valid for a given PostgreSQL column type
func (d *Dialect) GetOperatorsForType(columnType string) []core.OperatorInfo {
	typeLower := strings.ToLower(columnType)

	// Common comparison operators for all types
	common := []core.OperatorInfo{
		{Text: "=", InsertText: "= $0", Description: "Equal to"},
		{Text: "<>", InsertText: "<> $0", Description: "Not equal to"},
		{Text: "IS NULL", InsertText: "IS NULL", Description: "Value is null"},
		{Text: "IS NOT NULL", InsertText: "IS NOT NULL", Description: "Value is not null"},
		{Text: "IN", InsertText: "IN ($0)", Description: "Matches any value in list"},
	}

	// Array types - check BEFORE numeric/text since integer[], text[] etc contain base type names
	if isArrayType(typeLower) {
		return append(common,
			core.OperatorInfo{Text: "@>", InsertText: "@> ARRAY[$0]", Description: "Contains all elements"},
			core.OperatorInfo{Text: "<@", InsertText: "<@ ARRAY[$0]", Description: "Is contained by"},
			core.OperatorInfo{Text: "&&", InsertText: "&& ARRAY[$0]", Description: "Arrays overlap (share elements)"},
			core.OperatorInfo{Text: "||", InsertText: "|| ARRAY[$0]", Description: "Concatenate arrays"},
			core.OperatorInfo{Text: "ANY", InsertText: "= ANY($0)", Description: "Equals any array element"},
			core.OperatorInfo{Text: "ALL", InsertText: "= ALL($0)", Description: "Equals all array elements"},
		)
	}

	// JSON/JSONB types
	if isJSONType(typeLower) {
		return append(common,
			core.OperatorInfo{Text: "->", InsertText: "-> '$0'", Description: "Get JSON object field (as JSON)"},
			core.OperatorInfo{Text: "->>", InsertText: "->> '$0'", Description: "Get JSON object field (as text)"},
			core.OperatorInfo{Text: "#>", InsertText: "#> '{$0}'", Description: "Get JSON value at path (as JSON)"},
			core.OperatorInfo{Text: "#>>", InsertText: "#>> '{$0}'", Description: "Get JSON value at path (as text)"},
			core.OperatorInfo{Text: "@>", InsertText: "@> '$0'", Description: "JSON contains"},
			core.OperatorInfo{Text: "<@", InsertText: "<@ '$0'", Description: "JSON is contained by"},
			core.OperatorInfo{Text: "?", InsertText: "? '$0'", Description: "Key exists"},
			core.OperatorInfo{Text: "?|", InsertText: "?| ARRAY['$0']", Description: "Any key exists"},
			core.OperatorInfo{Text: "?&", InsertText: "?& ARRAY['$0']", Description: "All keys exist"},
		)
	}

	// Numeric types
	if isNumericType(typeLower) {
		return append(common,
			core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Less than"},
			core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "Greater than"},
			core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
			core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
			core.OperatorInfo{Text: "BETWEEN", InsertText: "BETWEEN $1 AND $0", Description: "Within range (inclusive)"},
		)
	}

	// Boolean types
	if isBooleanType(typeLower) {
		return append(common,
			core.OperatorInfo{Text: "IS TRUE", InsertText: "IS TRUE", Description: "Value is true"},
			core.OperatorInfo{Text: "IS FALSE", InsertText: "IS FALSE", Description: "Value is false"},
			core.OperatorInfo{Text: "IS NOT TRUE", InsertText: "IS NOT TRUE", Description: "Value is not true (false or null)"},
			core.OperatorInfo{Text: "IS NOT FALSE", InsertText: "IS NOT FALSE", Description: "Value is not false (true or null)"},
		)
	}

	// Text types (including char, varchar, text)
	if isTextType(typeLower) {
		return append(common,
			core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Less than (alphabetically)"},
			core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "Greater than (alphabetically)"},
			core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
			core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
			core.OperatorInfo{Text: "LIKE", InsertText: "LIKE '$0'", Description: "Pattern match (% = any, _ = one char)"},
			core.OperatorInfo{Text: "ILIKE", InsertText: "ILIKE '%$0%'", Description: "Case-insensitive pattern match"},
			core.OperatorInfo{Text: "NOT LIKE", InsertText: "NOT LIKE '$0'", Description: "Does not match pattern"},
			core.OperatorInfo{Text: "NOT ILIKE", InsertText: "NOT ILIKE '%$0%'", Description: "Case-insensitive not match"},
			core.OperatorInfo{Text: "SIMILAR TO", InsertText: "SIMILAR TO '$0'", Description: "SQL regex pattern match"},
			core.OperatorInfo{Text: "~", InsertText: "~ '$0'", Description: "POSIX regex match"},
			core.OperatorInfo{Text: "~*", InsertText: "~* '$0'", Description: "POSIX regex match (case-insensitive)"},
			core.OperatorInfo{Text: "!~", InsertText: "!~ '$0'", Description: "POSIX regex not match"},
			core.OperatorInfo{Text: "!~*", InsertText: "!~* '$0'", Description: "POSIX regex not match (case-insensitive)"},
		)
	}

	// Date/time types
	if isDateTimeType(typeLower) {
		return append(common,
			core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Before"},
			core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "After"},
			core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "On or before"},
			core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "On or after"},
			core.OperatorInfo{Text: "BETWEEN", InsertText: "BETWEEN $1 AND $0", Description: "Within date range (inclusive)"},
		)
	}

	// UUID type
	if strings.Contains(typeLower, "uuid") {
		return common
	}

	// Default: comparison operators
	return append(common,
		core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Less than"},
		core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "Greater than"},
		core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
		core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
	)
}

func isNumericType(t string) bool {
	numerics := []string{"int", "integer", "smallint", "bigint", "decimal", "numeric", "real", "double", "float", "serial", "money"}
	for _, n := range numerics {
		if strings.Contains(t, n) {
			return true
		}
	}
	return false
}

func isBooleanType(t string) bool {
	return strings.Contains(t, "bool")
}

func isArrayType(t string) bool {
	return strings.HasSuffix(t, "[]") || strings.Contains(t, "array")
}

func isJSONType(t string) bool {
	return strings.Contains(t, "json")
}

func isTextType(t string) bool {
	texts := []string{"text", "char", "varchar", "character", "citext", "name"}
	for _, txt := range texts {
		if strings.Contains(t, txt) {
			return true
		}
	}
	return false
}

func isDateTimeType(t string) bool {
	datetimes := []string{"date", "time", "timestamp", "interval"}
	for _, dt := range datetimes {
		if strings.Contains(t, dt) {
			return true
		}
	}
	return false
}


// InferColumnsFromSubquery implements core.SQLDialect.
// PG inspect uses the grammar walker directly; this stub satisfies the interface.
func (d *Dialect) InferColumnsFromSubquery(_ antlr.Parser, _ core.Metadata, _ string) []core.Column {
	return nil
}

// WalkFromClause implements core.SQLDialect.
// PG inspect uses the grammar walker directly; this stub satisfies the interface.
func (d *Dialect) WalkFromClause(_ antlr.Parser, _ core.RelationRefListener) {}

func defaultReserved() map[string]bool {
	words := []string{
		"SELECT", "FROM", "WHERE", "GROUP", "BY", "ORDER", "HAVING", "WITH", "AS",
		"INSERT", "UPDATE", "DELETE", "JOIN", "LEFT", "RIGHT", "FULL", "INNER", "OUTER",
		"ON", "USING", "DISTINCT", "ALL", "UNION", "EXCEPT", "INTERSECT", "LIMIT", "OFFSET",
		"FETCH", "ONLY", "INTO", "VALUES", "RETURNING", "AND", "OR", "NOT",
		"SET",
	}
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

func defaultKeywords() []string {
	return []string{
		"SELECT", "FROM", "WHERE", "GROUP BY", "ORDER BY", "HAVING", "WITH", "AS",
		"INSERT", "UPDATE", "DELETE", "JOIN", "LEFT JOIN", "RIGHT JOIN", "FULL JOIN", "INNER JOIN",
		"ON", "USING", "DISTINCT", "UNION", "EXCEPT", "INTERSECT", "LIMIT", "OFFSET",
		"FETCH", "ONLY", "VALUES", "RETURNING",
	}
}

func makeQuotedTokenTypes() map[int]bool {
	return map[int]bool{
		pg.PostgreSQLLexerQuotedIdentifier:                    true,
		pg.PostgreSQLLexerInvalidQuotedIdentifier:             true,
		pg.PostgreSQLLexerUnicodeQuotedIdentifier:             true,
		pg.PostgreSQLLexerInvalidUnicodeQuotedIdentifier:      true,
		pg.PostgreSQLLexerUnterminatedQuotedIdentifier:        true,
		pg.PostgreSQLLexerUnterminatedUnicodeQuotedIdentifier: true,
	}
}

func makeIdentifierTokenTypes() map[int]bool {
	return map[int]bool{
		pg.PostgreSQLLexerIdentifier:                          true,
		pg.PostgreSQLLexerQuotedIdentifier:                    true,
		pg.PostgreSQLLexerUnicodeQuotedIdentifier:             true,
		pg.PostgreSQLLexerUnterminatedQuotedIdentifier:        true,
		pg.PostgreSQLLexerUnterminatedUnicodeQuotedIdentifier: true,
		pg.PostgreSQLLexerInvalidQuotedIdentifier:             true,
		pg.PostgreSQLLexerInvalidUnicodeQuotedIdentifier:      true,
	}
}

func normalizePostgreSQLQuotedIdentifier(tokenText string) string {
	if len(tokenText) >= 2 && tokenText[0] == '"' && tokenText[len(tokenText)-1] == '"' {
		inner := tokenText[1 : len(tokenText)-1]
		return strings.ReplaceAll(inner, "\"\"", "\"")
	}
	return tokenText
}

func unquote(s string) string {
	if len(s) < 2 {
		return strings.ToLower(s)
	}
	if (s[0] == '\'' || s[0] == '"') && s[0] == s[len(s)-1] {
		return strings.ToLower(s[1 : len(s)-1])
	}
	return strings.ToLower(s)
}

func escapeDoubleQuotes(s string) string {
	return strings.ReplaceAll(s, "\"", "\"\"")
}
