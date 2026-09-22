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

// operatorFamily groups PostgreSQL types that accept the same operators. Keys
// are the bare type name format_type reports, matched exactly, so no type can
// be claimed by another whose spelling it happens to contain.
//
// The groups come from what the server answers, not from what the names
// suggest: point has no "=" at all, json has neither equality nor the
// containment operators jsonb has, and the seven geometric types share only
// "<->" between them.
var operatorFamily = map[string]string{
	"boolean": "boolean",

	"smallint": "numeric", "integer": "numeric", "bigint": "numeric",
	"decimal": "numeric", "numeric": "numeric", "real": "numeric",
	"double precision": "numeric", "money": "numeric",
	"smallserial": "numeric", "serial": "numeric", "bigserial": "numeric",

	"text": "text", "character varying": "text", "character": "text",
	"\"char\"": "text", "name": "text", "citext": "text",

	"date": "datetime", "time without time zone": "datetime",
	"time with time zone": "datetime", "timestamp without time zone": "datetime",
	"timestamp with time zone": "datetime", "interval": "datetime",

	// pg_type.typname spellings. format_type reports the SQL name above, but a
	// catalog built from the internal name, or written by hand, carries these.
	"bool": "boolean",
	"int2": "numeric", "int4": "numeric", "int8": "numeric",
	"float4": "numeric", "float8": "numeric",
	"serial2": "numeric", "serial4": "numeric", "serial8": "numeric",
	"varchar": "text", "bpchar": "text", "char": "text",
	"timestamp": "datetime", "timestamptz": "datetime",
	"time": "datetime", "timetz": "datetime",
	"varbit": "comparable",

	"json":  "json",
	"jsonb": "jsonb",

	"uuid": "identifier", "bytea": "bytea", "xml": "bare",
	"tsvector": "textsearch", "tsquery": "bare",
	"bit": "comparable", "bit varying": "comparable",

	"int4range": "range", "int8range": "range", "numrange": "range",
	"daterange": "range", "tsrange": "range", "tstzrange": "range",
	"int4multirange": "range", "int8multirange": "range", "nummultirange": "range",
	"datemultirange": "range", "tsmultirange": "range", "tstzmultirange": "range",

	"inet": "network", "cidr": "network",
	"macaddr": "comparable", "macaddr8": "comparable",

	"point": "geometric", "line": "geometric", "lseg": "geometric",
	"box": "geometric", "path": "geometric", "polygon": "geometric",
	"circle": "geometric",
}

// baseTypeName reduces what format_type reports to the name operatorFamily is
// keyed on, and says whether the column is an array of it. The parameter list
// sits inside the name for some types ("timestamp(3) without time zone"), so it
// is removed rather than truncated at.
func baseTypeName(columnType string) (name string, isArray bool) {
	name = strings.ToLower(strings.TrimSpace(columnType))
	if trimmed, cut := strings.CutSuffix(name, "[]"); cut {
		name, isArray = trimmed, true
	}
	for {
		open := strings.Index(name, "(")
		if open == -1 {
			break
		}
		close := strings.Index(name[open:], ")")
		if close == -1 {
			name = name[:open]
			break
		}
		name = name[:open] + name[open+close+1:]
	}
	name = strings.Join(strings.Fields(name), " ")
	// A type the search path does not cover arrives schema qualified.
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	return strings.TrimSpace(name), isArray
}

// Operators every type accepts, and the groups that only some do. Kept apart so
// a family states what it has rather than repeating the whole list.
func nullTests() []core.OperatorInfo {
	return []core.OperatorInfo{
		{Text: "IS NULL", InsertText: "IS NULL", Description: "Value is null"},
		{Text: "IS NOT NULL", InsertText: "IS NOT NULL", Description: "Value is not null"},
	}
}

func equality() []core.OperatorInfo {
	return []core.OperatorInfo{
		{Text: "=", InsertText: "= $0", Description: "Equal to"},
		{Text: "<>", InsertText: "<> $0", Description: "Not equal to"},
		{Text: "IN", InsertText: "IN ($0)", Description: "Matches any value in list"},
	}
}

func ordering(lessDesc, greaterDesc string) []core.OperatorInfo {
	return []core.OperatorInfo{
		{Text: "<", InsertText: "< $0", Description: lessDesc},
		{Text: ">", InsertText: "> $0", Description: greaterDesc},
		{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
		{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
		{Text: "BETWEEN", InsertText: "BETWEEN $1 AND $0", Description: "Within range (inclusive)"},
	}
}

func ops(groups ...[]core.OperatorInfo) []core.OperatorInfo {
	var out []core.OperatorInfo
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// GetOperatorsForType returns operators valid for a given PostgreSQL column type
func (d *Dialect) GetOperatorsForType(columnType string) []core.OperatorInfo {
	name, isArray := baseTypeName(columnType)
	if isArray {
		return ops(nullTests(), equality(), []core.OperatorInfo{
			{Text: "@>", InsertText: "@> ARRAY[$0]", Description: "Contains all elements"},
			{Text: "<@", InsertText: "<@ ARRAY[$0]", Description: "Is contained by"},
			{Text: "&&", InsertText: "&& ARRAY[$0]", Description: "Arrays overlap (share elements)"},
			{Text: "||", InsertText: "|| ARRAY[$0]", Description: "Concatenate arrays"},
			{Text: "ANY", InsertText: "= ANY($0)", Description: "Equals any array element"},
			{Text: "ALL", InsertText: "= ALL($0)", Description: "Equals all array elements"},
		})
	}

	switch operatorFamily[name] {
	case "boolean":
		return ops(nullTests(), equality(), []core.OperatorInfo{
			{Text: "IS TRUE", InsertText: "IS TRUE", Description: "Value is true"},
			{Text: "IS FALSE", InsertText: "IS FALSE", Description: "Value is false"},
			{Text: "IS NOT TRUE", InsertText: "IS NOT TRUE", Description: "Value is not true (false or null)"},
			{Text: "IS NOT FALSE", InsertText: "IS NOT FALSE", Description: "Value is not false (true or null)"},
		})

	case "numeric":
		return ops(nullTests(), equality(), ordering("Less than", "Greater than"))

	case "text":
		return ops(nullTests(), equality(), ordering("Less than (alphabetically)", "Greater than (alphabetically)"), []core.OperatorInfo{
			{Text: "LIKE", InsertText: "LIKE '$0'", Description: "Pattern match (% = any, _ = one char)"},
			{Text: "ILIKE", InsertText: "ILIKE '%$0%'", Description: "Case-insensitive pattern match"},
			{Text: "NOT LIKE", InsertText: "NOT LIKE '$0'", Description: "Does not match pattern"},
			{Text: "NOT ILIKE", InsertText: "NOT ILIKE '%$0%'", Description: "Case-insensitive not match"},
			{Text: "SIMILAR TO", InsertText: "SIMILAR TO '$0'", Description: "SQL regex pattern match"},
			{Text: "~", InsertText: "~ '$0'", Description: "POSIX regex match"},
			{Text: "~*", InsertText: "~* '$0'", Description: "POSIX regex match (case-insensitive)"},
			{Text: "!~", InsertText: "!~ '$0'", Description: "POSIX regex not match"},
			{Text: "!~*", InsertText: "!~* '$0'", Description: "POSIX regex not match (case-insensitive)"},
		})

	case "datetime":
		return ops(nullTests(), equality(), ordering("Before", "After"))

	// json holds text, so the server gives it no equality, no ordering and none
	// of the containment operators jsonb has. Only the extraction ones work.
	case "json":
		return ops(nullTests(), []core.OperatorInfo{
			{Text: "->", InsertText: "-> '$0'", Description: "Get JSON object field (as JSON)"},
			{Text: "->>", InsertText: "->> '$0'", Description: "Get JSON object field (as text)"},
			{Text: "#>", InsertText: "#> '{$0}'", Description: "Get JSON value at path (as JSON)"},
			{Text: "#>>", InsertText: "#>> '{$0}'", Description: "Get JSON value at path (as text)"},
		})

	case "jsonb":
		return ops(nullTests(), equality(), ordering("Less than", "Greater than"), []core.OperatorInfo{
			{Text: "->", InsertText: "-> '$0'", Description: "Get JSON object field (as JSON)"},
			{Text: "->>", InsertText: "->> '$0'", Description: "Get JSON object field (as text)"},
			{Text: "#>", InsertText: "#> '{$0}'", Description: "Get JSON value at path (as JSON)"},
			{Text: "#>>", InsertText: "#>> '{$0}'", Description: "Get JSON value at path (as text)"},
			{Text: "@>", InsertText: "@> '$0'", Description: "Contains"},
			{Text: "<@", InsertText: "<@ '$0'", Description: "Is contained by"},
			{Text: "?", InsertText: "? '$0'", Description: "Key exists"},
			{Text: "?|", InsertText: "?| ARRAY['$0']", Description: "Any key exists"},
			{Text: "?&", InsertText: "?& ARRAY['$0']", Description: "All keys exist"},
		})

	case "range":
		return ops(nullTests(), equality(), ordering("Less than", "Greater than"), []core.OperatorInfo{
			{Text: "@>", InsertText: "@> $0", Description: "Contains range or element"},
			{Text: "<@", InsertText: "<@ $0", Description: "Is contained by"},
			{Text: "&&", InsertText: "&& $0", Description: "Ranges overlap"},
			{Text: "-|-", InsertText: "-|- $0", Description: "Ranges are adjacent"},
		})

	case "network":
		return ops(nullTests(), equality(), ordering("Less than", "Greater than"), []core.OperatorInfo{
			{Text: "<<", InsertText: "<< $0", Description: "Is contained within subnet"},
			{Text: "<<=", InsertText: "<<= $0", Description: "Is contained within subnet or equals"},
			{Text: ">>", InsertText: ">> $0", Description: "Contains subnet"},
			{Text: ">>=", InsertText: ">>= $0", Description: "Contains subnet or equals"},
			{Text: "&&", InsertText: "&& $0", Description: "Either contains the other"},
		})

	// The seven geometric types share only "<->": point has no "=", polygon no
	// "=", path and lseg and line no "~=". Offering what all of them define
	// leaves some out rather than offering any that would not run.
	case "geometric":
		return ops(nullTests(), []core.OperatorInfo{
			{Text: "<->", InsertText: "<-> $0", Description: "Distance between"},
		})

	case "bytea":
		return ops(nullTests(), equality(), ordering("Less than", "Greater than"), []core.OperatorInfo{
			{Text: "LIKE", InsertText: "LIKE '$0'", Description: "Pattern match"},
			{Text: "||", InsertText: "|| $0", Description: "Concatenate"},
		})

	case "textsearch":
		return ops(nullTests(), equality(), ordering("Less than", "Greater than"), []core.OperatorInfo{
			{Text: "@@", InsertText: "@@ to_tsquery('$0')", Description: "Matches text search query"},
		})

	// Ordering a uuid runs, but it answers no question anyone asks, and a
	// suggestion list is where an operator nobody wants costs something.
	case "identifier":
		return ops(nullTests(), equality())

	case "bare":
		return ops(nullTests())

	// An unlisted name is a user type: an enum, a domain, or one introspection
	// reported before this map knew it. Enums and domains over an ordinary base
	// take equality and ordering, which is what the default gives them.
	default:
		return ops(nullTests(), equality(), ordering("Less than", "Greater than"))
	}
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
		"AND", "OR", "NOT", "ASC", "DESC", "IS", "IN", "LIKE", "BETWEEN",
		"CASE", "WHEN", "THEN", "ELSE", "END",
		"NULL", "TRUE", "FALSE", "EXISTS", "ALL", "CAST", "INTERVAL",
		// What a DDL statement acts on.
		"TABLE", "TEMPORARY TABLE", "VIEW", "MATERIALIZED VIEW",
		"INDEX", "UNIQUE INDEX", "SCHEMA", "DATABASE", "FUNCTION", "PROCEDURE",
		"TRIGGER", "SEQUENCE", "TYPE", "EXTENSION", "ROLE", "USER",
		"CONFLICT", "ILIKE",
		// The words a statement opens with, which is what an empty buffer takes.
		"CREATE", "ALTER", "DROP", "TRUNCATE", "EXPLAIN", "MERGE",
		"GRANT", "REVOKE", "SET", "SHOW", "VACUUM", "ANALYZE",
		"BEGIN", "COMMIT", "ROLLBACK", "CALL",
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
