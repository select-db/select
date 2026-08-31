package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	antlr "github.com/antlr4-go/antlr/v4"
	core "github.com/selectDb/dialect/core"
	sqlite "github.com/selectDb/dialect/sqlite/parser"
)

// simpleRelationRefListener is a minimal implementation of core.RelationRefListener used
// internally to gather references and virtual tables when inferring columns.
type simpleRelationRefListener struct {
	refs          []core.RelationRef
	vtabs         []core.RelationRef
	defaultSchema string
	meta          core.Metadata
}

func (l *simpleRelationRefListener) GetReferences() []core.RelationRef       { return l.refs }
func (l *simpleRelationRefListener) GetVirtualTables() []core.RelationRef      { return l.vtabs }
func (l *simpleRelationRefListener) SetReferences(refs []core.RelationRef)   { l.refs = refs }
func (l *simpleRelationRefListener) SetVirtualTables(vtabs []core.RelationRef) { l.vtabs = vtabs }
func (l *simpleRelationRefListener) GetDefaultSchema() string                   { return l.defaultSchema }
func (l *simpleRelationRefListener) GetMeta() core.Metadata                     { return l.meta }

// Register the SQLite dialect on package init. The "sqlite3" driver is registered
// by the app (backend/app.go); dialect tests register it in schema_test.go.
func init() {
	core.Register("sqlite", NewDialect())
}

// Ensure Dialect implements dialect.SQLDialect interface at compile time
var _ core.SQLDialect = (*Dialect)(nil)

// Dialect implements dialect.SQLDialect for SQLite
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

// NewDialect creates a new SQLite dialect instance
func NewDialect() *Dialect {
	d := &Dialect{
		reservedKeywords:     make(map[string]bool),
		builtinFunctions:     []string{},
		defaultKeywords:      []string{},
		quotedTokenTypes:     make(map[int]bool),
		identifierTokenTypes: make(map[int]bool),
		joinKeywords:         []int{},
		endOfClauseKeywords:  []int{},
	}

	d.initializeKeywords()
	d.initializeBuiltinFunctions()
	d.initializeTokenTypes()

	return d
}

// initializeKeywords sets up SQLite reserved keywords
func (d *Dialect) initializeKeywords() {
	// SQLite reserved keywords
	keywords := []string{
		"ABORT", "ACTION", "ADD", "AFTER", "ALL", "ALTER", "ANALYZE", "AND", "AS", "ASC",
		"ATTACH", "AUTOINCREMENT", "BEFORE", "BEGIN", "BETWEEN", "BY", "CASCADE", "CASE",
		"CAST", "CHECK", "COLLATE", "COLUMN", "COMMIT", "CONFLICT", "CONSTRAINT", "CREATE",
		"CROSS", "CURRENT_DATE", "CURRENT_TIME", "CURRENT_TIMESTAMP", "DATABASE", "DEFAULT",
		"DEFERRABLE", "DEFERRED", "DELETE", "DESC", "DETACH", "DISTINCT", "DROP", "EACH",
		"ELSE", "END", "ESCAPE", "EXCEPT", "EXCLUSIVE", "EXISTS", "EXPLAIN", "FAIL", "FOR",
		"FOREIGN", "FROM", "FULL", "GLOB", "GROUP", "HAVING", "IF", "IGNORE", "IMMEDIATE",
		"IN", "INDEX", "INDEXED", "INITIALLY", "INNER", "INSERT", "INSTEAD", "INTERSECT",
		"INTO", "IS", "ISNULL", "JOIN", "KEY", "LEFT", "LIKE", "LIMIT", "MATCH", "NATURAL",
		"NO", "NOT", "NOTNULL", "NULL", "OF", "OFFSET", "ON", "OR", "ORDER", "OUTER",
		"PLAN", "PRAGMA", "PRIMARY", "QUERY", "RAISE", "RECURSIVE", "REFERENCES", "REGEXP",
		"REINDEX", "RELEASE", "RENAME", "REPLACE", "RESTRICT", "RIGHT", "ROLLBACK", "ROW",
		"ROWS", "SAVEPOINT", "SELECT", "SET", "TABLE", "TEMP", "TEMPORARY", "THEN", "TO",
		"TRANSACTION", "TRIGGER", "UNION", "UNIQUE", "UPDATE", "USING", "VACUUM", "VALUES",
		"VIEW", "VIRTUAL", "WHEN", "WHERE", "WITH", "WITHOUT",
	}

	for _, keyword := range keywords {
		d.reservedKeywords[strings.ToUpper(keyword)] = true
	}
}

// initializeBuiltinFunctions sets up SQLite built-in functions
func (d *Dialect) initializeBuiltinFunctions() {
	// SQLite built-in functions
	d.builtinFunctions = []string{
		"abs", "changes", "char", "coalesce", "glob", "ifnull", "instr", "hex", "last_insert_rowid",
		"length", "like", "likelihood", "likely", "load_extension", "lower", "ltrim", "max",
		"min", "nullif", "printf", "quote", "random", "randomblob", "replace", "round", "rtrim",
		"soundex", "sqlite_compileoption_get", "sqlite_compileoption_used", "sqlite_source_id",
		"sqlite_version", "substr", "total_changes", "trim", "typeof", "unlikely", "unicode",
		"upper", "zeroblob",
		// Date and time functions
		"date", "time", "datetime", "julianday", "strftime",
		// Aggregate functions
		"avg", "count", "group_concat", "sum", "total",
		// Window functions
		"row_number", "rank", "dense_rank", "percent_rank", "cume_dist", "ntile", "lag",
		"lead", "first_value", "last_value", "nth_value",
	}
}

// initializeTokenTypes sets up token type mappings
func (d *Dialect) initializeTokenTypes() {
	// SQLite token types from the parser
	d.quotedTokenTypes = map[int]bool{
		sqlite.SQLiteLexerSTRING_LITERAL: true,
		sqlite.SQLiteLexerIDENTIFIER:     true,
	}

	d.identifierTokenTypes = map[int]bool{
		sqlite.SQLiteLexerIDENTIFIER: true,
	}

	d.joinKeywords = []int{
		sqlite.SQLiteLexerFROM_,
		sqlite.SQLiteLexerJOIN_,
		sqlite.SQLiteLexerINNER_,
		sqlite.SQLiteLexerLEFT_,
		sqlite.SQLiteLexerRIGHT_,
		sqlite.SQLiteLexerFULL_,
		sqlite.SQLiteLexerCROSS_,
		sqlite.SQLiteLexerNATURAL_,
	}

	d.endOfClauseKeywords = []int{
		sqlite.SQLiteLexerFROM_,
		sqlite.SQLiteLexerWHERE_,
		sqlite.SQLiteLexerGROUP_,
		sqlite.SQLiteLexerHAVING_,
		sqlite.SQLiteLexerORDER_,
		sqlite.SQLiteLexerLIMIT_,
	}
}

// Name returns the dialect name
func (d *Dialect) Name() string {
	return "sqlite"
}

// OpenDB opens a SQLite database in defensive mode unless the DSN opts out.
// See withDefensiveMode for what that covers and what it does not.
func (d *Dialect) OpenDB(dsn string) (*sql.DB, error) {
	return sql.Open("sqlite3", withDefensiveMode(dsn))
}

// GetTables, GetViews, GetIndexes, GetTriggers, and GetStats are implemented in schema.go

// CreateLexer creates a new SQLite lexer
func (d *Dialect) CreateLexer(input string) antlr.Lexer {
	return sqlite.NewSQLiteLexer(antlr.NewInputStream(input))
}

// CreateParser creates a new SQLite parser
func (d *Dialect) CreateParser(stream antlr.TokenStream) antlr.Parser {
	return sqlite.NewSQLiteParser(stream)
}

// GetReservedKeywords returns the reserved keywords map
func (d *Dialect) GetReservedKeywords() map[string]bool {
	return d.reservedKeywords
}

// GetBuiltinFunctions returns the list of built-in functions
func (d *Dialect) GetBuiltinFunctions() []string {
	return d.builtinFunctions
}

// GetDefaultKeywords returns default keywords for completion
func (d *Dialect) GetDefaultKeywords() []string {
	return d.defaultKeywords
}

// SupportsFeature checks if SQLite supports a specific feature
func (d *Dialect) SupportsFeature(feature core.Feature) bool {
	switch feature {
	case core.FeatureCTE:
		return true
	case core.FeatureRecursiveCTE:
		return true
	case core.FeatureWindowFunctions:
		return true
	case core.FeatureMaterializedViews:
		return false // SQLite doesn't have materialized views
	case core.FeatureForeignTables:
		return false // SQLite doesn't have foreign tables
	case core.FeatureSchemas:
		return false // SQLite doesn't have schemas
	default:
		return false
	}
}

// NormalizeIdentifier normalizes an identifier according to SQLite rules
func (d *Dialect) NormalizeIdentifier(raw string) string {
	// Strip quotes if present
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		// Quoted identifier - preserve case and strip quotes
		unquoted := raw[1 : len(raw)-1]
		// Handle escaped quotes (doubled quotes)
		return strings.ReplaceAll(unquoted, `""`, `"`)
	}
	// SQLite is case-insensitive for unquoted identifiers
	return strings.ToLower(raw)
}

// QuoteIdentifierIfNeeded quotes an identifier if it contains special characters or is a reserved keyword
func (d *Dialect) QuoteIdentifierIfNeeded(name string, caretQuoted bool, reserved map[string]bool) string {
	if name == "" {
		return name
	}

	// Check if it's already quoted
	if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
		return name
	}

	// Check if it needs quoting
	needsQuoting := d.IsReservedKeyword(name)

	// Check if it contains special characters
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			needsQuoting = true
			break
		}
	}

	// Check if it starts with a digit
	if len(name) > 0 && unicode.IsDigit(rune(name[0])) {
		needsQuoting = true
	}

	if needsQuoting {
		return fmt.Sprintf(`"%s"`, name)
	}

	return name
}

// IsValidUnquotedIdentifier checks if a string is a valid unquoted identifier
func (d *Dialect) IsValidUnquotedIdentifier(s string) bool {
	if s == "" {
		return false
	}

	// Must start with letter or underscore
	first := rune(s[0])
	if !unicode.IsLetter(first) && first != '_' {
		return false
	}

	// Rest must be letters, digits, or underscores
	for _, r := range s[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}

	// Must not be a reserved keyword
	return !d.IsReservedKeyword(s)
}

// IsReservedKeyword checks if a word is a reserved keyword
func (d *Dialect) IsReservedKeyword(word string) bool {
	return d.reservedKeywords[strings.ToUpper(word)]
}



























// InferColumnsFromSubquery implements the core.SQLDialect interface for SQLite.
// We heuristically parse the SELECT list of the subquery to extract simple column names
// (e.g., SELECT c1, c2 FROM t2). For each name, we look up type/nullability in metadata.
func (d *Dialect) InferColumnsFromSubquery(parser antlr.Parser, meta core.Metadata, defaultSchema string) []core.Column {
	// Try to read tokens from the underlying CommonTokenStream
	stream := parser.GetTokenStream()
	cts, ok := stream.(*antlr.CommonTokenStream)
	if !ok {
		return nil
	}
	cts.Fill()
	tokens := cts.GetAllTokens()

	// Token types we'll need
	selectTok := sqlite.SQLiteLexerSELECT_
	fromTok := sqlite.SQLiteLexerFROM_
	commaTok := sqlite.SQLiteLexerCOMMA
	dotTok := sqlite.SQLiteLexerDOT

	// FROM references for resolving qualified select-list items (e.g. t.c1)
	var fromRefs []core.RelationRef
	for idx := 0; idx < len(tokens); idx++ {
		if tokens[idx].GetTokenType() != selectTok {
			continue
		}
		selStart := tokens[idx].GetTokenIndex()
		lastIdx := tokens[len(tokens)-1].GetTokenIndex()
		selText := cts.GetTextFromInterval(antlr.NewInterval(selStart, lastIdx))
		subLexer := d.CreateLexer(selText)
		subStream := antlr.NewCommonTokenStream(subLexer, antlr.TokenDefaultChannel)
		subParser := d.CreateParser(subStream)
		tl := &simpleRelationRefListener{defaultSchema: defaultSchema, meta: meta}
		d.WalkFromClause(subParser, tl)
		fromRefs = tl.refs
		break
	}

	// Helper: append column by name with type inferred from metadata (by first match)
	inferCol := func(name string) core.Column {
		normalized := d.NormalizeIdentifier(name)
		for _, s := range meta.Schemas {
			for _, t := range s.Tables {
				for _, c := range t.Columns {
					if d.NormalizeIdentifier(c.Name) == normalized {
						return core.Column{Name: normalized, Type: c.Type, Nullable: c.Nullable}
					}
				}
			}
		}
		return core.Column{Name: normalized, Type: "unknown", Nullable: true}
	}

	// Find SELECT, then collect identifiers until FROM
	cols := []core.Column{}
	inSelectList := false
	aliasSet := map[string]bool{}
	commaCount := 0
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		tt := tok.GetTokenType()
		if tt == selectTok {
			inSelectList = true
			continue
		}
		if inSelectList {
			if tt == fromTok {
				break
			}
			// IDENTIFIERs represent select items; handle qualified names (a.b), then aliases:
			// AS IDENTIFIER | bare IDENTIFIER alias.
			if tt == sqlite.SQLiteLexerIDENTIFIER {
				j := i
				segments := []string{tokens[j].GetText()}
				for j+2 < len(tokens) &&
					tokens[j+1].GetTokenType() == dotTok &&
					tokens[j+2].GetTokenType() == sqlite.SQLiteLexerIDENTIFIER {
					j += 2
					segments = append(segments, tokens[j].GetText())
				}

				aliasName := ""
				consumeExtra := j
				if consumeExtra+2 < len(tokens) &&
					tokens[consumeExtra+1].GetTokenType() == sqlite.SQLiteLexerAS_ &&
					tokens[consumeExtra+2].GetTokenType() == sqlite.SQLiteLexerIDENTIFIER {
					aliasName = tokens[consumeExtra+2].GetText()
					i = consumeExtra + 2
				} else if consumeExtra+1 < len(tokens) &&
					tokens[consumeExtra+1].GetTokenType() == sqlite.SQLiteLexerIDENTIFIER {
					aliasName = tokens[consumeExtra+1].GetText()
					i = consumeExtra + 1
				} else {
					i = j
				}

				var col core.Column
				if len(segments) >= 2 {
					quals := make([]string, len(segments)-1)
					for k := 0; k < len(segments)-1; k++ {
						quals[k] = d.NormalizeIdentifier(segments[k])
					}
					colName := d.NormalizeIdentifier(segments[len(segments)-1])
					resolved := core.ResolveColumnFromRelationRefs(quals, colName, fromRefs, meta, d)
					if len(resolved) == 1 {
						col = resolved[0]
					} else {
						col = core.Column{Name: colName, Type: "unknown", Nullable: true}
					}
				} else {
					name := segments[0]
					target := name
					if aliasName != "" {
						target = aliasName
						aliasSet[d.NormalizeIdentifier(aliasName)] = true
					}
					col = inferCol(target)
				}

				if aliasName != "" && len(segments) >= 2 {
					col.Name = d.NormalizeIdentifier(aliasName)
					aliasSet[col.Name] = true
				}

				cols = append(cols, col)
				continue
			}
			// Skip commas and other tokens
			if tt == commaTok {
				commaCount++
				continue
			}
			// If STAR, try to expand using FROM sources (tables, subqueries, joins)
			if tt == sqlite.SQLiteLexerSTAR {
				// Use the dialect walker to discover sources and gather columns
				// Build a fresh parser starting at the SELECT to avoid leading parentheses
				selStart := 0
				for idx := 0; idx < len(tokens); idx++ {
					if tokens[idx].GetTokenType() == selectTok {
						selStart = tokens[idx].GetTokenIndex()
						break
					}
				}
				selText := cts.GetTextFromInterval(antlr.NewInterval(selStart, tokens[len(tokens)-1].GetTokenIndex()))
				subLexer := d.CreateLexer(selText)
				subStream := antlr.NewCommonTokenStream(subLexer, antlr.TokenDefaultChannel)
				subParser := d.CreateParser(subStream)

				tl := &simpleRelationRefListener{defaultSchema: defaultSchema, meta: meta}
				d.WalkFromClause(subParser, tl)

				// Prefer immediate projection sources:
				// - If FROM has subquery aliases (virtual tables) with columns, use those columns
				// - Otherwise, fall back to physical tables referenced at this level
				dedup := map[string]bool{}
				result := []core.Column{}
				if len(tl.vtabs) > 0 {
					for _, vt := range tl.vtabs {
						for _, c := range vt.Columns {
							name := d.NormalizeIdentifier(c.Name)
							if !dedup[name] {
								dedup[name] = true
								result = append(result, core.Column{Name: name, Type: c.Type, Nullable: c.Nullable})
							}
						}
					}
					if len(result) > 0 {
						return result
					}
				}
				for _, r := range tl.refs {
					colsFrom := core.GetColumnsForTableAsColumns(meta, r.Schema, r.Table, d)
					for _, c := range colsFrom {
						name := d.NormalizeIdentifier(c.Name)
						if !dedup[name] {
							dedup[name] = true
							result = append(result, core.Column{Name: name, Type: c.Type, Nullable: c.Nullable})
						}
					}
				}
				if len(result) > 0 {
					return result
				}
				return nil
			}
		}
	}
	// If aliases were present, prefer aliases only
	if len(aliasSet) > 0 {
		filtered := []core.Column{}
		for _, c := range cols {
			if aliasSet[d.NormalizeIdentifier(c.Name)] {
				filtered = append(filtered, c)
			}
		}
		cols = filtered
	}
	// Truncate to the number of select items (commas + 1)
	expectedN := commaCount + 1
	if expectedN > 0 && len(cols) > expectedN {
		cols = cols[:expectedN]
	}
	return cols
}



// GetOperatorsForType returns operators valid for a given SQLite column type
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

	// Numeric types (INTEGER, REAL, NUMERIC)
	if strings.Contains(typeLower, "int") || strings.Contains(typeLower, "real") ||
		strings.Contains(typeLower, "numeric") || strings.Contains(typeLower, "float") ||
		strings.Contains(typeLower, "double") {
		return append(common,
			core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Less than"},
			core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "Greater than"},
			core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
			core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
			core.OperatorInfo{Text: "BETWEEN", InsertText: "BETWEEN $1 AND $0", Description: "Within range (inclusive)"},
		)
	}

	// Boolean (SQLite stores as INTEGER 0/1)
	if strings.Contains(typeLower, "bool") {
		return append(common,
			core.OperatorInfo{Text: "IS", InsertText: "IS $0", Description: "Test boolean value"},
			core.OperatorInfo{Text: "IS NOT", InsertText: "IS NOT $0", Description: "Test not boolean value"},
		)
	}

	// Text types (TEXT, CHAR, VARCHAR, CLOB)
	if strings.Contains(typeLower, "text") || strings.Contains(typeLower, "char") ||
		strings.Contains(typeLower, "clob") || strings.Contains(typeLower, "varchar") {
		return append(common,
			core.OperatorInfo{Text: "<", InsertText: "< $0", Description: "Less than (alphabetically)"},
			core.OperatorInfo{Text: ">", InsertText: "> $0", Description: "Greater than (alphabetically)"},
			core.OperatorInfo{Text: "<=", InsertText: "<= $0", Description: "Less than or equal"},
			core.OperatorInfo{Text: ">=", InsertText: ">= $0", Description: "Greater than or equal"},
			core.OperatorInfo{Text: "LIKE", InsertText: "LIKE '$0'", Description: "Pattern match (% = any, _ = one char)"},
			core.OperatorInfo{Text: "NOT LIKE", InsertText: "NOT LIKE '$0'", Description: "Does not match pattern"},
			core.OperatorInfo{Text: "GLOB", InsertText: "GLOB '$0'", Description: "Unix glob pattern match (* = any)"},
			core.OperatorInfo{Text: "REGEXP", InsertText: "REGEXP '$0'", Description: "Regular expression match"},
			core.OperatorInfo{Text: "MATCH", InsertText: "MATCH '$0'", Description: "FTS full-text search match"},
		)
	}

	// BLOB type
	if strings.Contains(typeLower, "blob") {
		return common
	}

	// Date/time (SQLite stores as TEXT, REAL, or INTEGER)
	if strings.Contains(typeLower, "date") || strings.Contains(typeLower, "time") {
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

// WalkFromClause walks the FROM clause using the SQLite parser
func (d *Dialect) WalkFromClause(parser antlr.Parser, listener core.RelationRefListener) {
	sqliteParser := parser.(*sqlite.SQLiteParser)

	// Pull context from the listener via interface methods
	existingVtabs := listener.GetVirtualTables()
	defaultSchema := listener.GetDefaultSchema()
	if defaultSchema == "" {
		defaultSchema = "main"
	}
	meta := listener.GetMeta()

	sqliteListener := &relationRefListener{
		BaseSQLiteParserListener: &sqlite.BaseSQLiteParserListener{},
		refs:                     []core.RelationRef{},
		vtabs:                    existingVtabs,
		defaultSchema:            defaultSchema,
		meta:                     meta,
		dialect:                  d,
		level:                    0,
	}

	// SQLite parser expects a complete statement, so we need to walk the entire select statement
	// and then filter for FROM clause references
	antlr.ParseTreeWalkerDefault.Walk(sqliteListener, sqliteParser.Select_stmt())

	// Copy results to the provided listener via interface
	listener.SetReferences(sqliteListener.refs)
	listener.SetVirtualTables(sqliteListener.vtabs)
}

// ============================================
// ANTLR TREE LISTENER
// ============================================

// relationRefListener walks the FROM clause parse tree
type relationRefListener struct {
	*sqlite.BaseSQLiteParserListener
	refs          []core.RelationRef
	vtabs         []core.RelationRef
	defaultSchema string
	meta          core.Metadata
	dialect       *Dialect
	level         int // grammar nesting of Table_or_subquery nodes at current FROM
	subqueryDepth int // semantic depth of subquery contexts for nesting level
	depthStack    []bool
}

func (l *relationRefListener) EnterTable_or_subquery(ctx *sqlite.Table_or_subqueryContext) {
	// Track grammar nesting
	if _, ok := ctx.GetParent().(*sqlite.Table_or_subqueryContext); ok {
		l.level++
	}

	// Manage subquery semantic depth: if this node is a subquery, increase depth for its subtree
	isSubq := ctx.Select_stmt() != nil
	if isSubq {
		l.subqueryDepth++
	}
	l.depthStack = append(l.depthStack, isSubq)

	// Skip if this is part of a JOIN clause (we'll handle it in EnterJoin_clause)
	if l.isInJoinClause(ctx) {
		return
	}

	switch {
	case ctx.Table_name() != nil:
		// Physical table reference
		tableName := ctx.Table_name()
		ref := core.RelationRef{
			Schema:        l.defaultSchema,
			ScopeStartPos: -1, // Will be set by ParseFromClause
			ScopeEndPos:   -1, // Will be set by ParseFromClause
		}

		// Handle qualified table names (schema.table)
		if ctx.Schema_name() != nil {
			ref.Schema = l.dialect.NormalizeIdentifier(ctx.Schema_name().GetText())
		}
		ref.Table = l.dialect.NormalizeIdentifier(tableName.Any_name().GetText())

		// Skip JOIN keywords that might be parsed as table names
		if l.isJoinKeyword(ref.Table) {
			return
		}

		// Check if this is a CTE (virtual table) - CTEs have no schema
		if l.isCTE(ref.Table) {
			ref.Schema = ""
		}

		// Handle table alias
		if ctx.Table_alias() != nil {
			alias, colNames := l.normalizeTableAlias(ctx.Table_alias())
			// Skip if alias is a JOIN keyword (SQLite allows keywords as identifiers)
			if !l.isJoinKeyword(alias) {
				if len(colNames) > 0 {
					cols := make([]core.Column, len(colNames))
					for i, colName := range colNames {
						cols[i] = core.Column{Name: colName, Type: "unknown", Nullable: true}
					}
					// Only add virtual table if it's not already a CTE
					if !l.isCTE(alias) {
						l.vtabs = append(l.vtabs, core.RelationRef{Table: alias, Columns: cols, IsVirtual: true})
					}
				} else {
					ref.Alias = alias
				}
			}
		}

		// Assign semantic nesting level
		ref.NestingLevel = l.subqueryDepth
		l.refs = append(l.refs, ref)

	case ctx.Select_stmt() != nil:
		// Subquery
		if ctx.Table_alias() != nil {
			alias, explicitColNames := l.normalizeTableAlias(ctx.Table_alias())
			aliasNesting := l.subqueryDepth - 1
			if aliasNesting < 0 {
				aliasNesting = 0
			}
			vt := core.RelationRef{Table: alias, NestingLevel: aliasNesting, IsVirtual: true}

			if len(explicitColNames) > 0 {
				cols := make([]core.Column, len(explicitColNames))
				for i, colName := range explicitColNames {
					cols[i] = core.Column{Name: colName, Type: "unknown", Nullable: true}
				}
				vt.Columns = cols
			} else {
				// Create a parser to call InferColumnsFromSubquery
				// Get the text from the stream to preserve whitespace
				selectStmt := ctx.Select_stmt()
				startIdx := selectStmt.GetStart().GetTokenIndex()
				stopIdx := selectStmt.GetStop().GetTokenIndex()
				subqueryText := selectStmt.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(startIdx, stopIdx))

				subLexer := l.dialect.CreateLexer(subqueryText)
				subStream := antlr.NewCommonTokenStream(subLexer, antlr.TokenDefaultChannel)
				subParser := l.dialect.CreateParser(subStream)
				vt.Columns = l.dialect.InferColumnsFromSubquery(subParser, l.meta, l.defaultSchema)
			}

			l.vtabs = append(l.vtabs, vt)
			// For subqueries, the alias IS the table name, so we don't set Alias
			l.refs = append(l.refs, core.RelationRef{
				Schema:        "",
				Table:         alias,
				Alias:         "",
				IsVirtual:     true,
				ScopeStartPos: -1, // Will be set by ParseFromClause
				ScopeEndPos:   -1, // Will be set by ParseFromClause
				NestingLevel:  aliasNesting,
			})
		}
	}
}

func (l *relationRefListener) ExitTable_or_subquery(ctx *sqlite.Table_or_subqueryContext) {
	// Pop subquery depth if this node was a subquery
	if len(l.depthStack) > 0 {
		wasSubq := l.depthStack[len(l.depthStack)-1]
		l.depthStack = l.depthStack[:len(l.depthStack)-1]
		if wasSubq && l.subqueryDepth > 0 {
			l.subqueryDepth--
		}
	}
	if _, ok := ctx.GetParent().(*sqlite.Table_or_subqueryContext); ok && l.level > 0 {
		l.level--
	}
}

func (l *relationRefListener) EnterJoin_clause(ctx *sqlite.Join_clauseContext) {
	// Handle JOIN clauses - get all table references from the join
	// Skip the first table_or_subquery as it's the left side of the join
	// and will be handled by the select_core context
	tableOrSubqueries := ctx.AllTable_or_subquery()
	if len(tableOrSubqueries) == 0 {
		return
	}

	// Process all table_or_subquery contexts (including the first one,
	// since it won't be processed elsewhere in a JOIN clause)
	for _, tos := range tableOrSubqueries {
		if tos.Table_name() != nil {
			// Physical table reference
			tableName := tos.Table_name()
			ref := core.RelationRef{
				Schema:        l.defaultSchema,
				ScopeStartPos: -1, // Will be set by ParseFromClause
				ScopeEndPos:   -1, // Will be set by ParseFromClause
			}

			// Handle qualified table names (schema.table)
			if tos.Schema_name() != nil {
				ref.Schema = l.dialect.NormalizeIdentifier(tos.Schema_name().GetText())
			}
			ref.Table = l.dialect.NormalizeIdentifier(tableName.Any_name().GetText())

			// Skip JOIN keywords that might be parsed as table names
			if l.isJoinKeyword(ref.Table) {
				continue
			}

			// Check if this is a CTE (virtual table) - CTEs have no schema
			if l.isCTE(ref.Table) {
				ref.Schema = ""
			}

			// Handle table alias
			if tos.Table_alias() != nil {
				alias, colNames := l.normalizeTableAlias(tos.Table_alias())
				// Skip if alias is a JOIN keyword (SQLite allows keywords as identifiers)
				if !l.isJoinKeyword(alias) {
					if len(colNames) > 0 {
						cols := make([]core.Column, len(colNames))
						for i, colName := range colNames {
							cols[i] = core.Column{Name: colName, Type: "unknown", Nullable: true}
						}
						// Only add virtual table if it's not already a CTE
						if !l.isCTE(alias) {
							l.vtabs = append(l.vtabs, core.RelationRef{Table: alias, Columns: cols, IsVirtual: true})
						}
					} else {
						ref.Alias = alias
					}
				}
			}

			l.refs = append(l.refs, ref)
		} else if tos.Select_stmt() != nil {
			// Subquery
			if tos.Table_alias() != nil {
				alias, explicitColNames := l.normalizeTableAlias(tos.Table_alias())
				vt := core.RelationRef{Table: alias, IsVirtual: true}

				if len(explicitColNames) > 0 {
					cols := make([]core.Column, len(explicitColNames))
					for i, colName := range explicitColNames {
						cols[i] = core.Column{Name: colName, Type: "unknown", Nullable: true}
					}
					vt.Columns = cols
				} else {
					// Create a parser to call InferColumnsFromSubquery
					// Get the text from the stream to preserve whitespace
					selectStmt := tos.Select_stmt()
					startIdx := selectStmt.GetStart().GetTokenIndex()
					stopIdx := selectStmt.GetStop().GetTokenIndex()
					subqueryText := selectStmt.GetParser().GetTokenStream().GetTextFromInterval(antlr.NewInterval(startIdx, stopIdx))

					subLexer := l.dialect.CreateLexer(subqueryText)
					subStream := antlr.NewCommonTokenStream(subLexer, antlr.TokenDefaultChannel)
					subParser := l.dialect.CreateParser(subStream)
					vt.Columns = l.dialect.InferColumnsFromSubquery(subParser, l.meta, l.defaultSchema)
				}

				l.vtabs = append(l.vtabs, vt)
				// For subqueries, the alias IS the table name, so we don't set Alias
				l.refs = append(l.refs, core.RelationRef{
					Schema:        "",
					Table:         alias,
					Alias:         "",
					IsVirtual:     true,
					ScopeStartPos: -1, // Will be set by ParseFromClause
					ScopeEndPos:   -1, // Will be set by ParseFromClause
				})
			}
		}
	}
}

// isCTE checks if a table name is a CTE (Common Table Expression)
func (l *relationRefListener) isCTE(tableName string) bool {
	for _, vtab := range l.vtabs {
		if vtab.Table == tableName {
			return true
		}
	}
	return false
}

// isJoinKeyword checks if a table name is actually a JOIN keyword
func (l *relationRefListener) isJoinKeyword(tableName string) bool {
	joinKeywords := []string{"cross", "inner", "left", "right", "full", "natural", "join"}
	for _, keyword := range joinKeywords {
		if l.dialect.NormalizeIdentifier(tableName) == keyword {
			return true
		}
	}
	return false
}

// isInJoinClause checks if a Table_or_subquery is inside a JOIN clause
func (l *relationRefListener) isInJoinClause(ctx *sqlite.Table_or_subqueryContext) bool {
	// Walk up the parse tree to see if we're inside a Join_clause
	parent := ctx.GetParent()
	for parent != nil {
		if _, ok := parent.(*sqlite.Join_clauseContext); ok {
			return true
		}
		parent = parent.GetParent()
	}
	return false
}

func (l *relationRefListener) normalizeTableAlias(ctx sqlite.ITable_aliasContext) (string, []string) {
	if ctx == nil {
		return "", nil
	}

	normalized := l.dialect.NormalizeIdentifier(ctx.GetText())
	// Filter out reserved keywords (keyword pollution fix)
	// Keywords are typically stored in uppercase, so check both normalized and uppercase
	reservedKeywords := l.dialect.GetReservedKeywords()
	aliasUpper := strings.ToUpper(normalized)
	if reservedKeywords == nil || (!reservedKeywords[normalized] && !reservedKeywords[aliasUpper]) {
		return normalized, nil
	}
	// If it's a keyword, return empty string
	return "", nil // SQLite doesn't support column aliases in table aliases
}
