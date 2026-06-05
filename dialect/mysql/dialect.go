package mysql

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	core "github.com/selectDb/dialect/core"
	mysql "github.com/selectDb/dialect/mysql/parser"

	antlr "github.com/antlr4-go/antlr/v4"
	_ "github.com/go-sql-driver/mysql"
)

// Register the MySQL dialect on package init
func init() {
	core.Register("mysql", NewDialect())
}

// Ensure Dialect implements dialect.SQLDialect interface at compile time
var _ core.SQLDialect = (*Dialect)(nil)

// Dialect implements dialect.SQLDialect for MySQL
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

func (d *Dialect) SetAnalyzer(a core.Analyzer) { d.analyzer = a }

// NewDialect creates a new MySQL dialect
func NewDialect() *Dialect {
	return &Dialect{
		reservedKeywords:     defaultReserved(),
		builtinFunctions:     mysql.GetBuiltinFunctions(),
		defaultKeywords:      defaultKeywords(),
		quotedTokenTypes:     makeQuotedTokenTypes(),
		identifierTokenTypes: makeIdentifierTokenTypes(),
		joinKeywords: []int{
			mysql.MySQLLexerFROM_SYMBOL, mysql.MySQLLexerJOIN_SYMBOL,
			mysql.MySQLLexerLEFT_SYMBOL, mysql.MySQLLexerRIGHT_SYMBOL,
			mysql.MySQLLexerINNER_SYMBOL, mysql.MySQLLexerOUTER_SYMBOL,
			mysql.MySQLLexerCROSS_SYMBOL,
		},
		endOfClauseKeywords: []int{
			mysql.MySQLLexerFROM_SYMBOL, mysql.MySQLLexerORDER_SYMBOL, mysql.MySQLLexerWHERE_SYMBOL,
		},
	}
}

func (d *Dialect) Name() string {
	return "mysql"
}

func (d *Dialect) OpenDB(dsn string) (*sql.DB, error) {
	return sql.Open("mysql", dsn)
}

func (d *Dialect) CreateLexer(input string) antlr.Lexer {
	return mysql.NewMySQLLexer(antlr.NewInputStream(input))
}

func (d *Dialect) CreateParser(stream antlr.TokenStream) antlr.Parser {
	parser := mysql.NewMySQLParser(stream)
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
	case core.FeatureCTE, core.FeatureRecursiveCTE, core.FeatureWindowFunctions:
		return true
	case core.FeatureMaterializedViews, core.FeatureForeignTables, core.FeatureSchemas:
		return false // MySQL doesn't support these features
	default:
		return false
	}
}

func (d *Dialect) NormalizeIdentifier(raw string) string {
	if len(raw) >= 2 && raw[0] == '`' && raw[len(raw)-1] == '`' {
		return normalizeMySQLQuotedIdentifier(raw)
	}
	return strings.ToLower(raw)
}

func (d *Dialect) QuoteIdentifierIfNeeded(name string, caretQuoted bool, reserved map[string]bool) string {
	if caretQuoted {
		return name
	}
	if strings.ToLower(name) != name {
		return fmt.Sprintf("`%s`", escapeBackticks(name))
	}
	if reserved[strings.ToUpper(name)] {
		return fmt.Sprintf("`%s`", escapeBackticks(name))
	}
	if !d.IsValidUnquotedIdentifier(name) {
		return fmt.Sprintf("`%s`", escapeBackticks(name))
	}
	return name
}

func (d *Dialect) IsValidUnquotedIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := rune(s[0])
	if !unicode.IsLetter(first) && first != '_' && first != '$' {
		return false
	}
	for _, ch := range s[1:] {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '$' {
			return false
		}
	}
	return true
}





// Syntax token queries for context-aware parsing






















// WalkFromClause walks the FROM clause using the MySQL parser
func (d *Dialect) WalkFromClause(parser antlr.Parser, listener core.RelationRefListener) {
	// Placeholder implementation for MySQL
	// TODO: Implement MySQL-specific FROM clause walking
	// This should create a MySQL-specific listener and walk the parse tree
	// For now, we just return empty results to maintain interface compliance
}

// InferColumnsFromSubquery implements the core.SQLDialect interface
func (d *Dialect) InferColumnsFromSubquery(parser antlr.Parser, meta core.Metadata, defaultSchema string) []core.Column {
	// Placeholder implementation for MySQL
	// TODO: Implement MySQL-specific column inference from subqueries
	// This should parse the subquery and extract column information
	// For now, we just return empty results to maintain interface compliance
	return []core.Column{}
}



// GetOperatorsForType returns operators valid for a given MySQL column type
func (d *Dialect) GetOperatorsForType(columnType string) []core.OperatorInfo {
	typeLower := strings.ToLower(columnType)

	// Common comparison operators for all types
	common := []core.OperatorInfo{
		{Text: "=", InsertText: "= $0", Description: "Equal to"},
		{Text: "<>", InsertText: "<> $0", Description: "Not equal to"},
		{Text: "!=", InsertText: "!= $0", Description: "Not equal to (alternative)"},
		{Text: "IS NULL", InsertText: "IS NULL", Description: "Value is null"},
		{Text: "IS NOT NULL", InsertText: "IS NOT NULL", Description: "Value is not null"},
		{Text: "IN", InsertText: "IN ($0)", Description: "Matches any value in list"},
	}

	// Numeric types
	if strings.Contains(typeLower, "int") || strings.Contains(typeLower, "decimal") ||
		strings.Contains(typeLower, "numeric") || strings.Contains(typeLower, "float") ||
		strings.Contains(typeLower, "double") || strings.Contains(typeLower, "real") {
		return append(common,
			core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Less than"},
			core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "Greater than"},
			core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
			core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
			core.OperatorInfo{Text: "BETWEEN", InsertText: "BETWEEN $1 AND $0", Description: "Within range (inclusive)"},
			core.OperatorInfo{Text: "<=>", InsertText: "<=> $0", Description: "NULL-safe equal"},
		)
	}

	// Boolean (TINYINT(1) or BOOLEAN)
	if strings.Contains(typeLower, "bool") || typeLower == "tinyint(1)" {
		return append(common,
			core.OperatorInfo{Text: "IS TRUE", InsertText: "IS TRUE", Description: "Value is true"},
			core.OperatorInfo{Text: "IS FALSE", InsertText: "IS FALSE", Description: "Value is false"},
			core.OperatorInfo{Text: "IS NOT TRUE", InsertText: "IS NOT TRUE", Description: "Value is not true"},
			core.OperatorInfo{Text: "IS NOT FALSE", InsertText: "IS NOT FALSE", Description: "Value is not false"},
		)
	}

	// Text types
	if strings.Contains(typeLower, "char") || strings.Contains(typeLower, "text") ||
		strings.Contains(typeLower, "varchar") || strings.Contains(typeLower, "blob") {
		return append(common,
			core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Less than (alphabetically)"},
			core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "Greater than (alphabetically)"},
			core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
			core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
			core.OperatorInfo{Text: "LIKE", InsertText: "LIKE '$0'", Description: "Pattern match (% = any, _ = one char)"},
			core.OperatorInfo{Text: "NOT LIKE", InsertText: "NOT LIKE '$0'", Description: "Does not match pattern"},
			core.OperatorInfo{Text: "REGEXP", InsertText: "REGEXP '$0'", Description: "Regular expression match"},
			core.OperatorInfo{Text: "RLIKE", InsertText: "RLIKE '$0'", Description: "Regular expression match (alias)"},
			core.OperatorInfo{Text: "SOUNDS LIKE", InsertText: "SOUNDS LIKE '$0'", Description: "Phonetic match (Soundex)"},
		)
	}

	// JSON type
	if strings.Contains(typeLower, "json") {
		return append(common,
			core.OperatorInfo{Text: "->", InsertText: "-> '$0'", Description: "Get JSON object field (as JSON)"},
			core.OperatorInfo{Text: "->>", InsertText: "->> '$0'", Description: "Get JSON object field (as text)"},
			core.OperatorInfo{Text: "JSON_CONTAINS", InsertText: "JSON_CONTAINS($0)", Description: "Check if JSON contains value"},
			core.OperatorInfo{Text: "JSON_OVERLAPS", InsertText: "JSON_OVERLAPS($0)", Description: "Check if JSONs share elements"},
		)
	}

	// Date/time types
	if strings.Contains(typeLower, "date") || strings.Contains(typeLower, "time") ||
		strings.Contains(typeLower, "year") {
		return append(common,
			core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Before"},
			core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "After"},
			core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "On or before"},
			core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "On or after"},
			core.OperatorInfo{Text: "BETWEEN", InsertText: "BETWEEN $1 AND $0", Description: "Within date range (inclusive)"},
		)
	}

	// Default: comparison operators
	return append(common,
		core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Less than"},
		core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "Greater than"},
		core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
		core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
	)
}

// ============================================
// HELPER FUNCTIONS
// ============================================

func defaultReserved() map[string]bool {
	words := []string{
		"SELECT", "FROM", "WHERE", "GROUP", "BY", "ORDER", "HAVING", "WITH", "AS",
		"INSERT", "UPDATE", "DELETE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER",
		"ON", "USING", "DISTINCT", "ALL", "UNION", "EXCEPT", "INTERSECT", "LIMIT", "OFFSET",
		"INTO", "VALUES", "AND", "OR", "NOT",
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
		"INSERT", "UPDATE", "DELETE", "JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "OUTER JOIN",
		"ON", "USING", "DISTINCT", "UNION", "LIMIT", "OFFSET",
		"VALUES",
	}
}

func makeQuotedTokenTypes() map[int]bool {
	return map[int]bool{
		mysql.MySQLLexerBACK_TICK_QUOTED_ID: true,
		mysql.MySQLLexerDOUBLE_QUOTED_TEXT:  true,
		mysql.MySQLLexerSINGLE_QUOTED_TEXT:  true,
	}
}

func makeIdentifierTokenTypes() map[int]bool {
	return map[int]bool{
		mysql.MySQLLexerIDENTIFIER:          true,
		mysql.MySQLLexerBACK_TICK_QUOTED_ID: true,
		mysql.MySQLLexerDOUBLE_QUOTED_TEXT:  true,
		mysql.MySQLLexerSINGLE_QUOTED_TEXT:  true,
	}
}

func normalizeMySQLQuotedIdentifier(tokenText string) string {
	if len(tokenText) >= 2 && tokenText[0] == '`' && tokenText[len(tokenText)-1] == '`' {
		inner := tokenText[1 : len(tokenText)-1]
		return strings.ReplaceAll(inner, "``", "`")
	}
	return tokenText
}

func escapeBackticks(s string) string {
	return strings.ReplaceAll(s, "`", "``")
}
