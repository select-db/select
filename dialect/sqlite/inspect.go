package sqlite

import (
	"sort"
	"strings"

	core "github.com/selectDb/dialect/core"
	sqlite "github.com/selectDb/dialect/sqlite/parser"

	antlr "github.com/antlr4-go/antlr/v4"
)

// Inspector analyzes SQL statements and extracts structured information
type Inspector struct {
	dialect *Dialect
	meta    core.Metadata
}

// NewInspector creates a new SQLite statement inspector
func NewInspector(dialect *Dialect, meta core.Metadata) *Inspector {
	return &Inspector{
		dialect: dialect,
		meta:    meta,
	}
}

// Inspect implements core.SQLDialect.Inspect by delegating to a fresh Inspector.
func (d *Dialect) Inspect(meta core.Metadata, sql string) []core.InspectStatement {
	return NewInspector(d, meta).Inspect(sql)
}

// ============================================
// HELPER METHODS - Reduce repeated patterns
// ============================================

// normalizeEquals compares two identifiers after normalization
func (i *Inspector) normalizeEquals(a, b string) bool {
	return i.dialect.NormalizeIdentifier(a) == i.dialect.NormalizeIdentifier(b)
}

// Inspect analyzes SQL and returns structured results for each statement
func (i *Inspector) Inspect(sql string) []core.InspectStatement {
	lexer := i.dialect.CreateLexer(sql)
	tokenStream := antlr.NewCommonTokenStream(lexer, 0)
	tokenStream.Fill() // pre-fill for compound-operator detection
	parser := sqlite.NewSQLiteParser(tokenStream)
	parser.RemoveErrorListeners()

	root := parser.Parse()
	if root == nil {
		return nil
	}

	stmtLists := root.AllSql_stmt_list()
	if len(stmtLists) == 0 {
		return nil
	}

	var results []core.InspectStatement
	idx := 0
	for idx < len(stmtLists) {
		// Collect consecutive stmt_lists connected by compound operators (UNION/INTERSECT/EXCEPT).
		// The SQLite grammar emits each UNION branch as a separate sql_stmt_list at the top level.
		group := []sqlite.ISql_stmt_listContext{stmtLists[idx]}
		for idx+1 < len(stmtLists) && hasCompoundOperatorBetween(tokenStream, stmtLists[idx], stmtLists[idx+1]) {
			idx++
			group = append(group, stmtLists[idx])
		}

		if len(group) > 1 {
			result := i.mergeCompoundSelectGroup(group)
			if result != nil {
				results = append(results, *result)
			}
		} else {
			for _, stmt := range group[0].AllSql_stmt() {
				result := i.inspectStatement(stmt)
				if result != nil {
					results = append(results, *result)
				}
			}
		}
		idx++
	}

	return results
}

// hasCompoundOperatorBetween reports whether UNION/INTERSECT/EXCEPT tokens appear between two parse-tree nodes.
// The compound operator is the last token of the first stmt_list, so we scan from stopIdx (inclusive).
func hasCompoundOperatorBetween(tokens *antlr.CommonTokenStream, a, b antlr.ParserRuleContext) bool {
	stopIdx := a.GetStop().GetTokenIndex()
	startIdx := b.GetStart().GetTokenIndex()
	allTokens := tokens.GetAllTokens()
	for ti := stopIdx; ti < startIdx && ti < len(allTokens); ti++ {
		tok := allTokens[ti]
		if tok.GetChannel() != antlr.TokenDefaultChannel {
			continue
		}
		switch strings.ToUpper(tok.GetText()) {
		case "UNION", "INTERSECT", "EXCEPT":
			return true
		}
	}
	return false
}

// mergeCompoundSelectGroup merges consecutive stmt_lists that are compound SELECT branches.
func (i *Inspector) mergeCompoundSelectGroup(group []sqlite.ISql_stmt_listContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpSelect}
	for _, stmtList := range group {
		for _, stmt := range stmtList.AllSql_stmt() {
			if selectStmt := stmt.Select_stmt(); selectStmt != nil {
				branch := i.inspectSelect(selectStmt)
				if branch == nil {
					continue
				}
				result.Tables = core.MergeInspectTables(result.Tables, branch.Tables)
				result.Fields = core.MergeInspectFields(result.Fields, branch.Fields)
				result.Where = core.MergeInspectFields(result.Where, branch.Where)
				result.Subqueries = append(result.Subqueries, branch.Subqueries...)
			}
		}
	}
	return result
}

// inspectStatement dispatches to the appropriate handler based on statement type
func (i *Inspector) inspectStatement(stmt sqlite.ISql_stmtContext) *core.InspectStatement {
	if stmt == nil {
		return nil
	}

	if selectStmt := stmt.Select_stmt(); selectStmt != nil {
		return i.inspectSelect(selectStmt)
	}
	if insertStmt := stmt.Insert_stmt(); insertStmt != nil {
		return i.inspectInsert(insertStmt)
	}
	if updateStmt := stmt.Update_stmt(); updateStmt != nil {
		return i.inspectUpdate(updateStmt)
	}
	if deleteStmt := stmt.Delete_stmt(); deleteStmt != nil {
		return i.inspectDelete(deleteStmt)
	}
	if dropStmt := stmt.Drop_stmt(); dropStmt != nil {
		return i.inspectDrop(dropStmt)
	}
	if createStmt := stmt.Create_table_stmt(); createStmt != nil {
		return i.inspectCreate(createStmt)
	}
	if alterStmt := stmt.Alter_table_stmt(); alterStmt != nil {
		return i.inspectAlterTable(alterStmt)
	}

	return nil
}

// inspectSelect analyzes a SELECT statement, including UNION/INTERSECT/EXCEPT compounds.
func (i *Inspector) inspectSelect(selectStmt sqlite.ISelect_stmtContext) *core.InspectStatement {
	if selectStmt == nil {
		return nil
	}

	selectCores := selectStmt.AllSelect_core()
	if len(selectCores) == 0 {
		return nil
	}

	// CTEs are shared across all UNION/INTERSECT/EXCEPT branches.
	var ctes []core.RelationRef
	var cteSubqueries []core.InspectStatement
	if commonTableStmt := selectStmt.Common_table_stmt(); commonTableStmt != nil {
		ctes, cteSubqueries = i.extractCTEsWithSubqueries(commonTableStmt)
	}

	cteToSubqueryMap := make(map[string]*core.InspectStatement)
	for idx, cte := range ctes {
		if idx < len(cteSubqueries) {
			cteToSubqueryMap[i.dialect.NormalizeIdentifier(cte.Table)] = &cteSubqueries[idx]
		}
	}

	result := &core.InspectStatement{
		Operation:  core.InspectOpSelect,
		Subqueries: cteSubqueries,
	}
	for _, sub := range cteSubqueries {
		result.Tables = core.MergeInspectTables(result.Tables, sub.Tables)
	}

	for _, selectCore := range selectCores {
		branch := i.inspectSelectCore(selectCore, ctes, cteSubqueries, cteToSubqueryMap)
		result.Tables = core.MergeInspectTables(result.Tables, branch.Tables)
		result.Fields = core.MergeInspectFields(result.Fields, branch.Fields)
		result.Where = core.MergeInspectFields(result.Where, branch.Where)
		result.Subqueries = append(result.Subqueries, branch.Subqueries...)
	}

	return result
}

// inspectSelectCore processes a single select_core (one branch of a compound query).
func (i *Inspector) inspectSelectCore(
	selectCore sqlite.ISelect_coreContext,
	ctes []core.RelationRef,
	cteSubqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) core.InspectStatement {
	relationRefs, subqueryColumns := i.extractRelationRefs(selectCore)

	fromSubqueries := i.extractFromSubqueries(selectCore, cteToSubqueryMap)

	virtualTables := make(map[string]bool)
	for _, cte := range ctes {
		virtualTables[i.dialect.NormalizeIdentifier(cte.Table)] = true
	}
	for name := range subqueryColumns {
		virtualTables[i.dialect.NormalizeIdentifier(name)] = true
	}

	tables := i.convertRelationRefs(relationRefs, virtualTables)
	for _, subq := range fromSubqueries {
		tables = core.MergeInspectTables(tables, subq.Tables)
	}

	allSubqueries := fromSubqueries
	fields := i.extractSelectFieldsWithResolution(selectCore, relationRefs, ctes, subqueryColumns, allSubqueries, cteToSubqueryMap)

	where, whereSubqueries := i.extractWhereFields(selectCore, relationRefs)
	selectSubqueries := i.extractSelectListSubqueries(selectCore)

	subqueries := append(fromSubqueries, whereSubqueries...)
	subqueries = append(subqueries, selectSubqueries...)

	return core.InspectStatement{
		Tables:     tables,
		Fields:     fields,
		Where:      where,
		Subqueries: subqueries,
	}
}

// resolveQualifiedTableName extracts schema and table name from a qualified_table_name context.
func (i *Inspector) resolveQualifiedTableName(qtname sqlite.IQualified_table_nameContext) (schema, table string) {
	if qtname == nil {
		return "", ""
	}
	effectiveSchema := i.meta.CurrentSchema
	if effectiveSchema == "" {
		effectiveSchema = i.meta.DefaultSchema
	}
	schema = effectiveSchema
	if qtname.Schema_name() != nil {
		schema = i.dialect.NormalizeIdentifier(qtname.Schema_name().GetText())
	}
	if qtname.Table_name() != nil {
		table = i.dialect.NormalizeIdentifier(qtname.Table_name().Any_name().GetText())
	}
	return schema, table
}

// resolveInsertTarget extracts schema and table name from an INSERT statement.
func (i *Inspector) resolveInsertTarget(stmt sqlite.IInsert_stmtContext) (schema, table string) {
	effectiveSchema := i.meta.CurrentSchema
	if effectiveSchema == "" {
		effectiveSchema = i.meta.DefaultSchema
	}
	schema = effectiveSchema
	if stmt.Schema_name() != nil {
		schema = i.dialect.NormalizeIdentifier(stmt.Schema_name().GetText())
	}
	if stmt.Table_name() != nil {
		table = i.dialect.NormalizeIdentifier(stmt.Table_name().Any_name().GetText())
	}
	return schema, table
}

// inspectInsert analyzes an INSERT statement.
func (i *Inspector) inspectInsert(stmt sqlite.IInsert_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpInsert}

	schema, tableName := i.resolveInsertTarget(stmt)
	if tableName == "" {
		return result
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	// Collect explicitly listed target columns.
	colNames := stmt.AllColumn_name()
	if len(colNames) > 0 {
		for _, cn := range colNames {
			name := i.dialect.NormalizeIdentifier(cn.Any_name().GetText())
			result.Fields = append(result.Fields, core.InspectField{
				Name:   name,
				Table:  tableName,
				Schema: schema,
			})
		}
	} else {
		// No explicit column list, expand to all columns from metadata.
		for _, col := range core.GetColumnsForTableAsColumns(i.meta, schema, tableName, i.dialect) {
			result.Fields = append(result.Fields, core.InspectField{
				Name:   col.Name,
				Table:  tableName,
				Schema: schema,
			})
		}
	}

	// INSERT … SELECT: attach source as subquery when it has real tables.
	if selectStmt := stmt.Select_stmt(); selectStmt != nil {
		if sub := i.inspectSelect(selectStmt); sub != nil && len(sub.Tables) > 0 {
			result.Subqueries = append(result.Subqueries, *sub)
		}
	} else {
		// VALUES form: walk expressions for embedded subqueries.
		result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(stmt)...)
	}

	return result
}

// inspectUpdate analyzes an UPDATE statement.
func (i *Inspector) inspectUpdate(stmt sqlite.IUpdate_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpUpdate}

	schema, tableName := i.resolveQualifiedTableName(stmt.Qualified_table_name())
	if tableName == "" {
		return result
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	// SET column names, AllColumn_name() returns the LHS of each assignment.
	for _, cn := range stmt.AllColumn_name() {
		name := i.dialect.NormalizeIdentifier(cn.Any_name().GetText())
		result.Fields = append(result.Fields, core.InspectField{
			Name:   name,
			Table:  tableName,
			Schema: schema,
		})
	}

	targetRef := []core.RelationRef{{Table: tableName, Schema: schema}}

	// RHS expressions: scan for embedded subqueries, excluding the WHERE expr.
	allExprs := stmt.AllExpr()
	rhsExprs := allExprs
	if stmt.WHERE_() != nil && len(allExprs) > 0 {
		rhsExprs = allExprs[:len(allExprs)-1]
	}
	for _, expr := range rhsExprs {
		result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(expr)...)
	}

	// WHERE clause: the last Expr() is the WHERE condition when WHERE_ token is present.
	if stmt.WHERE_() != nil {
		if len(allExprs) > 0 {
			whereExpr := allExprs[len(allExprs)-1]
			where, whereSubqueries := i.extractWhereFieldsFromExpr(whereExpr, targetRef)
			result.Where = where
			result.Subqueries = append(result.Subqueries, whereSubqueries...)
		}
	}

	return result
}

// inspectDelete analyzes a DELETE statement.
func (i *Inspector) inspectDelete(stmt sqlite.IDelete_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpDelete}

	schema, tableName := i.resolveQualifiedTableName(stmt.Qualified_table_name())
	if tableName == "" {
		return result
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	if stmt.WHERE_() != nil && stmt.Expr() != nil {
		targetRef := []core.RelationRef{{Table: tableName, Schema: schema}}
		where, whereSubqueries := i.extractWhereFieldsFromExpr(stmt.Expr(), targetRef)
		result.Where = where
		result.Subqueries = whereSubqueries
	}

	return result
}

// inspectDrop analyzes a DROP TABLE/INDEX/VIEW/TRIGGER statement.
func (i *Inspector) inspectDrop(stmt sqlite.IDrop_stmtContext) *core.InspectStatement {
	// Only handle DROP TABLE; other object types don't need permission checks.
	if stmt.TABLE_() == nil {
		return nil
	}
	result := &core.InspectStatement{Operation: core.InspectOpDrop}
	effectiveSchema := i.meta.CurrentSchema
	if effectiveSchema == "" {
		effectiveSchema = i.meta.DefaultSchema
	}
	schema := effectiveSchema
	if stmt.Schema_name() != nil {
		schema = i.dialect.NormalizeIdentifier(stmt.Schema_name().GetText())
	}
	if stmt.Any_name() != nil {
		table := i.dialect.NormalizeIdentifier(stmt.Any_name().GetText())
		result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
	}
	return result
}

// inspectCreate analyzes a CREATE TABLE statement.
func (i *Inspector) inspectCreate(stmt sqlite.ICreate_table_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpCreate}
	effectiveSchema := i.meta.CurrentSchema
	if effectiveSchema == "" {
		effectiveSchema = i.meta.DefaultSchema
	}
	schema := effectiveSchema
	if stmt.Schema_name() != nil {
		schema = i.dialect.NormalizeIdentifier(stmt.Schema_name().GetText())
	}
	if stmt.Table_name() != nil {
		table := i.dialect.NormalizeIdentifier(stmt.Table_name().Any_name().GetText())
		result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
	}
	return result
}

// inspectAlterTable analyzes an ALTER TABLE statement.
func (i *Inspector) inspectAlterTable(stmt sqlite.IAlter_table_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpAlter}
	effectiveSchema := i.meta.CurrentSchema
	if effectiveSchema == "" {
		effectiveSchema = i.meta.DefaultSchema
	}
	schema := effectiveSchema
	if stmt.Schema_name() != nil {
		schema = i.dialect.NormalizeIdentifier(stmt.Schema_name().GetText())
	}
	tableNames := stmt.AllTable_name()
	if len(tableNames) > 0 {
		table := i.dialect.NormalizeIdentifier(tableNames[0].Any_name().GetText())
		result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
	}
	return result
}

// extractRelationRefs extracts table references from a SELECT statement
// Uses a listener to walk the select_core's FROM clause
func (i *Inspector) extractRelationRefs(selectCore sqlite.ISelect_coreContext) ([]core.RelationRef, map[string][]core.Column) {
	// Determine effective schema: use CurrentSchema if set, otherwise DefaultSchema
	effectiveSchema := i.meta.CurrentSchema
	if effectiveSchema == "" {
		effectiveSchema = i.meta.DefaultSchema
	}

	// Use a listener similar to the dialect's relationRefListener
	listener := &relationRefExtractorListener{
		BaseSQLiteParserListener: &sqlite.BaseSQLiteParserListener{},
		refs:                     []core.RelationRef{},
		vtabs:                    []core.RelationRef{},
		defaultSchema:            effectiveSchema,
		meta:                     i.meta,
		dialect:                  i.dialect,
		level:                    0,
		subqueryDepth:            0,
		depthStack:               []bool{},
	}

	// Walk the select_core to extract table references
	antlr.ParseTreeWalkerDefault.Walk(listener, selectCore)

	// Extract subquery columns from virtual tables
	// Subqueries in FROM clause are identified by having columns but being in vtabs
	// CTEs are handled separately in extractCTEsWithSubqueries
	subqueryColumns := make(map[string][]core.Column)
	for _, vtab := range listener.vtabs {
		// All virtual tables from the listener are subqueries (CTEs come from Common_table_stmt)
		if len(vtab.Columns) > 0 {
			subqueryColumns[vtab.Table] = vtab.Columns
		}
	}

	return listener.refs, subqueryColumns
}

// relationRefExtractorListener extracts table references from select_core
// Similar to relationRefListener in dialect.go but works on select_core directly
type relationRefExtractorListener struct {
	*sqlite.BaseSQLiteParserListener
	refs              []core.RelationRef
	vtabs             []core.RelationRef
	defaultSchema     string
	meta              core.Metadata
	dialect           *Dialect
	level             int
	subqueryDepth     int // depth from FROM-clause subqueries
	exprSubqueryDepth int // depth from expression subqueries (WHERE, HAVING, SELECT list)
	depthStack        []bool
}

// EnterSelect_stmt tracks expression-level subqueries (those inside WHERE, HAVING, SELECT list).
// FROM-clause subqueries are tracked via subqueryDepth in EnterTable_or_subquery instead.
func (l *relationRefExtractorListener) EnterSelect_stmt(ctx *sqlite.Select_stmtContext) {
	if _, ok := ctx.GetParent().(*sqlite.Table_or_subqueryContext); !ok {
		l.exprSubqueryDepth++
	}
}

func (l *relationRefExtractorListener) ExitSelect_stmt(ctx *sqlite.Select_stmtContext) {
	if _, ok := ctx.GetParent().(*sqlite.Table_or_subqueryContext); !ok {
		l.exprSubqueryDepth--
	}
}

func (l *relationRefExtractorListener) EnterTable_or_subquery(ctx *sqlite.Table_or_subqueryContext) {
	// Track grammar nesting
	if _, ok := ctx.GetParent().(*sqlite.Table_or_subqueryContext); ok {
		l.level++
	}

	// Manage subquery semantic depth
	isSubq := ctx.Select_stmt() != nil
	if isSubq {
		l.subqueryDepth++
	}
	l.depthStack = append(l.depthStack, isSubq)

	// Don't pull tables from inside expression subqueries (WHERE/HAVING/SELECT list) into outer scope.
	if l.exprSubqueryDepth > 0 {
		return
	}
	// Don't pull tables from inside FROM subqueries into outer scope.
	if l.subqueryDepth > 1 || (l.subqueryDepth == 1 && !isSubq) {
		return
	}

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
			ScopeStartPos: -1,
			ScopeEndPos:   -1,
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

		// Note: CTE detection happens later when we have the full CTE list
		// We can't check isCTE here because vtabs might not be fully populated yet
		// CTEs will be handled during field resolution

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

		ref.NestingLevel = l.subqueryDepth
		if tok := ctx.Table_name().Any_name().GetStart(); tok != nil {
			ref.Line = tok.GetLine()
			ref.Col = tok.GetColumn()
			ref.EndCol = tok.GetColumn() + len(tok.GetText())
		}
		l.refs = append(l.refs, ref)

	case ctx.Select_stmt() != nil:
		// Subquery - process even if no alias (use generated name)
		alias := ""
		explicitColNames := []string{}

		if ctx.Table_alias() != nil {
			alias, explicitColNames = l.normalizeTableAlias(ctx.Table_alias())
		}

		// If no alias, generate one for internal tracking (won't appear in final results)
		if alias == "" {
			alias = "_subquery_" + string(rune(len(l.vtabs)))
		}

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
			// Infer columns from subquery
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
		l.refs = append(l.refs, core.RelationRef{
			Schema:        "",
			Table:         alias,
			Alias:         "",
			ScopeStartPos: -1,
			ScopeEndPos:   -1,
			NestingLevel:  aliasNesting,
		})
	}
}

func (l *relationRefExtractorListener) ExitTable_or_subquery(ctx *sqlite.Table_or_subqueryContext) {
	// Pop subquery depth
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

func (l *relationRefExtractorListener) EnterJoin_clause(ctx *sqlite.Join_clauseContext) {
	// Don't process JOINs inside expression or FROM subqueries.
	if l.exprSubqueryDepth > 0 || l.subqueryDepth > 0 {
		return
	}

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
				ScopeStartPos: -1,
				ScopeEndPos:   -1,
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
			// Subquery - process even if no alias
			alias := ""
			explicitColNames := []string{}

			if tos.Table_alias() != nil {
				alias, explicitColNames = l.normalizeTableAlias(tos.Table_alias())
			}

			// If no alias, generate one for internal tracking
			if alias == "" {
				alias = "_subquery_" + string(rune(len(l.vtabs)))
			}

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
				ScopeStartPos: -1,
				ScopeEndPos:   -1,
				NestingLevel:  aliasNesting,
			})
		}
	}
}

// normalizeTableAlias extracts alias and column names from table_alias
func (l *relationRefExtractorListener) normalizeTableAlias(ctx sqlite.ITable_aliasContext) (string, []string) {
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

// isJoinKeyword checks if a table name is actually a JOIN keyword
func (l *relationRefExtractorListener) isJoinKeyword(tableName string) bool {
	joinKeywords := []string{"cross", "inner", "left", "right", "full", "natural", "join"}
	for _, keyword := range joinKeywords {
		if l.dialect.NormalizeIdentifier(tableName) == keyword {
			return true
		}
	}
	return false
}

// isCTE checks if a table name is a CTE
func (l *relationRefExtractorListener) isCTE(tableName string) bool {
	for _, vtab := range l.vtabs {
		if vtab.Table == tableName {
			return true
		}
	}
	return false
}

// isInJoinClause checks if a Table_or_subquery is inside a JOIN clause
func (l *relationRefExtractorListener) isInJoinClause(ctx *sqlite.Table_or_subqueryContext) bool {
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

// extractSelectFieldsWithResolution extracts fields and resolves virtual tables to underlying tables
func (i *Inspector) extractSelectFieldsWithResolution(
	selectCore sqlite.ISelect_coreContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	subqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	fields := i.extractSelectFields(selectCore, relationRefs, ctes, subqueryColumns, cteToSubqueryMap)

	// Build a map of virtual table -> underlying fields from subqueries
	// Use deterministic ordering: CTEs first, then subqueries in order
	virtualTableFields := make(map[string][]core.InspectField)

	// Map CTEs to their subquery results
	for idx, cte := range ctes {
		if idx < len(subqueries) {
			cteKey := i.dialect.NormalizeIdentifier(cte.Table)
			virtualTableFields[cteKey] = subqueries[idx].Fields
		}
	}

	// Map subquery aliases - need deterministic ordering
	// Collect subquery names in order first
	subqueryNames := make([]string, 0, len(subqueryColumns))
	for name := range subqueryColumns {
		subqueryNames = append(subqueryNames, name)
	}
	// Sort for deterministic ordering
	sort.Strings(subqueryNames)

	subqIdx := len(ctes)
	for _, name := range subqueryNames {
		if subqIdx < len(subqueries) {
			normalizedName := i.dialect.NormalizeIdentifier(name)
			virtualTableFields[normalizedName] = subqueries[subqIdx].Fields
			subqIdx++
		}
	}

	// Resolve fields that reference virtual tables
	var resolvedFields []core.InspectField
	for _, field := range fields {
		normalizedTable := i.dialect.NormalizeIdentifier(field.Table)
		if underlyingFields, ok := virtualTableFields[normalizedTable]; ok {
			// Find the matching field in the underlying query
			for _, uf := range underlyingFields {
				if i.normalizeEquals(uf.Name, field.Name) {
					resolvedField := core.InspectField{
						Name:   field.Name,
						Alias:  field.Alias,
						Table:  uf.Table,
						Schema: uf.Schema,
					}
					resolvedFields = append(resolvedFields, resolvedField)
					break
				}
			}
		} else {
			resolvedFields = append(resolvedFields, field)
		}
	}

	return resolvedFields
}

// extractSelectFields extracts fields from the SELECT clause
func (i *Inspector) extractSelectFields(
	selectCore sqlite.ISelect_coreContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	// Get result columns directly from select_core
	resultColumns := selectCore.AllResult_column()
	if len(resultColumns) == 0 {
		return nil
	}

	var fields []core.InspectField

	for _, resultCol := range resultColumns {
		// Handle SELECT * (case 1: just STAR)
		if resultCol.STAR() != nil && resultCol.Table_name() == nil {
			fields = append(fields, i.expandStar(relationRefs, ctes, subqueryColumns, cteToSubqueryMap)...)
			continue
		}

		// Handle table.* (case 2: table_name DOT STAR)
		if resultCol.Table_name() != nil && resultCol.DOT() != nil && resultCol.STAR() != nil {
			tableName := resultCol.Table_name().Any_name().GetText()
			normalizedTable := i.dialect.NormalizeIdentifier(tableName)
			fields = append(fields, i.expandQualifiedStar(normalizedTable, relationRefs, ctes, cteToSubqueryMap)...)
			continue
		}

		// Handle regular expressions (case 3: expr with optional alias)
		if resultCol.Expr() != nil {
			exprFields := i.extractFieldsFromExpr(resultCol.Expr(), relationRefs, ctes, subqueryColumns, cteToSubqueryMap)
			if len(exprFields) > 0 {
				// Only propagate the alias when the expression resolves to a single column
				// (e.g. "SELECT c1 AS alias"). For computed expressions like "c1 + c2 AS calc",
				// the alias names the result, not any individual source column.
				if len(exprFields) == 1 && resultCol.Column_alias() != nil {
					normalizedAlias := i.dialect.NormalizeIdentifier(resultCol.Column_alias().GetText())
					exprFields[0].Alias = &normalizedAlias
				}
				fields = append(fields, exprFields...)
			}
		}
	}

	return fields
}

// expandStar expands SELECT * to all columns from all tables
func (i *Inspector) expandStar(
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	var fields []core.InspectField

	for _, ref := range relationRefs {
		// Check if this is a CTE reference
		for _, cte := range ctes {
			if i.dialect.NormalizeIdentifier(cte.Table) == i.dialect.NormalizeIdentifier(ref.Table) {
				// Use the CTE's subquery result to resolve columns properly
				cteKey := i.dialect.NormalizeIdentifier(cte.Table)
				if cteResult, ok := cteToSubqueryMap[cteKey]; ok {
					// Use fields from the CTE's InspectStatement
					fields = append(fields, cteResult.Fields...)
				} else {
					// Fallback: resolve each column individually
					for _, col := range cte.Columns {
						resolvedField := i.resolveCTEColumnFromFields(col.Name, nil)
						if resolvedField != nil {
							fields = append(fields, *resolvedField)
						}
					}
				}
				continue
			}
		}

		// Check if this is a subquery reference
		if ref.Schema == "" && subqueryColumns != nil {
			if vtabCols, ok := subqueryColumns[ref.Table]; ok {
				for _, col := range vtabCols {
					fields = append(fields, core.InspectField{
						Name:   col.Name,
						Table:  ref.Table,
						Schema: i.meta.DefaultSchema,
					})
				}
				continue
			}
		}

		// Regular table - lookup in metadata
		tableCols := core.GetColumnsForTableAsColumns(i.meta, ref.Schema, ref.Table, i.dialect)
		for _, col := range tableCols {
			fields = append(fields, core.InspectField{
				Name:   col.Name,
				Table:  ref.Table,
				Schema: ref.Schema,
			})
		}
	}

	return fields
}

// expandQualifiedStar expands table.* to all columns from the specified table
func (i *Inspector) expandQualifiedStar(
	tablePrefix string,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	normalizedPrefix := i.dialect.NormalizeIdentifier(tablePrefix)

	// Find the matching table reference
	for _, ref := range relationRefs {
		tableName := ref.Alias
		if tableName == "" {
			tableName = ref.Table
		}
		if i.dialect.NormalizeIdentifier(tableName) != normalizedPrefix {
			continue
		}

		// Check if this is a CTE reference
		for _, cte := range ctes {
			if i.dialect.NormalizeIdentifier(cte.Table) == i.dialect.NormalizeIdentifier(ref.Table) {
				cteKey := i.dialect.NormalizeIdentifier(cte.Table)
				if cteResult, ok := cteToSubqueryMap[cteKey]; ok {
					// Use fields from the CTE's InspectStatement
					return cteResult.Fields
				}
				// Fallback: resolve each column individually
				var fields []core.InspectField
				for _, col := range cte.Columns {
					resolvedField := i.resolveCTEColumnFromFields(col.Name, nil)
					if resolvedField != nil {
						fields = append(fields, *resolvedField)
					}
				}
				return fields
			}
		}

		// Regular table
		tableCols := core.GetColumnsForTableAsColumns(i.meta, ref.Schema, ref.Table, i.dialect)
		var fields []core.InspectField
		for _, col := range tableCols {
			fields = append(fields, core.InspectField{
				Name:   col.Name,
				Table:  ref.Table,
				Schema: ref.Schema,
			})
		}
		return fields
	}

	return nil
}

// exprColumnRef holds a raw column reference found in an expression.
type exprColumnRef struct {
	name        string
	tablePrefix string // empty if unqualified
	line        int    // 1-based, 0 = unknown
	col         int    // 0-based
	endCol      int    // 0-based exclusive
}

// extractFieldsFromExpr extracts all column references from an expression (excluding subquery internals).
func (i *Inspector) extractFieldsFromExpr(
	expr sqlite.IExprContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	if expr == nil {
		return nil
	}

	listener := &exprColumnExtractorListener{
		BaseSQLiteParserListener: &sqlite.BaseSQLiteParserListener{},
		inspector:                i,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, expr)

	var fields []core.InspectField
	for _, col := range listener.columns {
		field := i.resolveColumn(col.name, col.tablePrefix, nil, relationRefs, ctes, subqueryColumns, cteToSubqueryMap)
		if field != nil {
			field.StartLine = col.line
			field.StartCol = col.col
			field.EndCol = col.endCol
			fields = append(fields, *field)
		}
	}
	return fields
}

// exprColumnExtractorListener collects all column references in an expression, not descending into subqueries.
type exprColumnExtractorListener struct {
	*sqlite.BaseSQLiteParserListener
	inspector     *Inspector
	columns       []exprColumnRef
	subqueryDepth int
}

func (l *exprColumnExtractorListener) EnterSelect_stmt(_ *sqlite.Select_stmtContext) {
	l.subqueryDepth++
}

func (l *exprColumnExtractorListener) ExitSelect_stmt(_ *sqlite.Select_stmtContext) {
	l.subqueryDepth--
}

func (l *exprColumnExtractorListener) EnterExpr(ctx *sqlite.ExprContext) {
	if ctx == nil || l.subqueryDepth > 0 {
		return
	}
	if colName := ctx.Column_name(); colName != nil {
		if anyName := colName.Any_name(); anyName != nil {
			columnName := l.inspector.dialect.NormalizeIdentifier(anyName.GetText())
			if columnName == "" {
				return
			}
			var tablePrefix string
			if ctx.Table_name() != nil && len(ctx.AllDOT()) > 0 {
				tablePrefix = l.inspector.dialect.NormalizeIdentifier(ctx.Table_name().Any_name().GetText())
			}
			ref := exprColumnRef{name: columnName, tablePrefix: tablePrefix}
			if tok := anyName.GetStart(); tok != nil {
				ref.line = tok.GetLine()
				ref.col = tok.GetColumn()
				ref.endCol = tok.GetColumn() + len(tok.GetText())
			}
			l.columns = append(l.columns, ref)
		}
	}
}

// resolveColumn resolves a column name to its source table
func (i *Inspector) resolveColumn(
	columnName string,
	tablePrefix string,
	alias *string,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) *core.InspectField {
	normalizedCol := i.dialect.NormalizeIdentifier(columnName)

	// If table prefix is specified, resolve using that
	if tablePrefix != "" {
		for _, ref := range relationRefs {
			refKey := ref.Alias
			if refKey == "" {
				refKey = ref.Table
			}
			if i.dialect.NormalizeIdentifier(refKey) == i.dialect.NormalizeIdentifier(tablePrefix) {
				// Check if this is a CTE
				for _, cte := range ctes {
					if i.dialect.NormalizeIdentifier(cte.Table) == i.dialect.NormalizeIdentifier(ref.Table) {
						return i.resolveCTEColumnFromFields(normalizedCol, cteToSubqueryMap[i.dialect.NormalizeIdentifier(cte.Table)])
					}
				}
				return &core.InspectField{
					Name:   normalizedCol,
					Alias:  alias,
					Table:  ref.Table,
					Schema: ref.Schema,
				}
			}
		}
	}

	// Search all table refs for the column
	for _, ref := range relationRefs {
		// Check CTEs first
		for _, cte := range ctes {
			if i.dialect.NormalizeIdentifier(cte.Table) == i.dialect.NormalizeIdentifier(ref.Table) {
				for _, col := range cte.Columns {
					if i.dialect.NormalizeIdentifier(col.Name) == normalizedCol {
						cteKey := i.dialect.NormalizeIdentifier(cte.Table)
						resolved := i.resolveCTEColumnFromFields(normalizedCol, cteToSubqueryMap[cteKey])
						if resolved != nil {
							resolved.Alias = alias
							return resolved
						}
					}
				}
			}
		}

		// Check subquery columns
		if ref.Schema == "" && subqueryColumns != nil {
			if vtabCols, ok := subqueryColumns[ref.Table]; ok {
				for _, col := range vtabCols {
					if i.dialect.NormalizeIdentifier(col.Name) == normalizedCol {
						return &core.InspectField{
							Name:   normalizedCol,
							Alias:  alias,
							Table:  ref.Table,
							Schema: i.meta.DefaultSchema,
						}
					}
				}
			}
		}

		// Check regular table
		tableCols := core.GetColumnsForTableAsColumns(i.meta, ref.Schema, ref.Table, i.dialect)
		for _, col := range tableCols {
			if i.dialect.NormalizeIdentifier(col.Name) == normalizedCol {
				return &core.InspectField{
					Name:   normalizedCol,
					Alias:  alias,
					Table:  ref.Table,
					Schema: ref.Schema,
				}
			}
		}
	}

	// Fallback: column not found in any table metadata.
	// Attribute to the sole real table in scope if unambiguous.
	var knownRefs []core.RelationRef
	var unknownRefs []core.RelationRef
	for _, ref := range relationRefs {
		if ref.Schema == "" {
			continue // virtual (CTE / subquery alias)
		}
		if core.TableExistsInMetadata(i.meta, ref.Schema, ref.Table, i.dialect) {
			knownRefs = append(knownRefs, ref)
		} else {
			unknownRefs = append(unknownRefs, ref)
		}
	}
	if len(knownRefs) == 1 {
		return &core.InspectField{Name: normalizedCol, Alias: alias, Table: knownRefs[0].Table, Schema: knownRefs[0].Schema}
	}
	if len(knownRefs) == 0 && len(unknownRefs) == 1 {
		// Unknown table, schema left blank so permission checker can deny.
		return &core.InspectField{Name: normalizedCol, Alias: alias, Table: unknownRefs[0].Table, Schema: ""}
	}

	return nil
}

// resolveCTEColumnFromFields resolves a CTE column to its underlying table using the CTE's InspectStatement
func (i *Inspector) resolveCTEColumnFromFields(columnName string, cteResult *core.InspectStatement) *core.InspectField {
	// If we have the CTE's InspectStatement, use its fields to resolve the column
	if cteResult != nil {
		normalizedCol := i.dialect.NormalizeIdentifier(columnName)
		for _, field := range cteResult.Fields {
			if i.dialect.NormalizeIdentifier(field.Name) == normalizedCol {
				return &core.InspectField{
					Name:   field.Name,
					Table:  field.Table,
					Schema: field.Schema,
				}
			}
		}
	}

	// Fallback: look for the column in metadata (for backwards compatibility)
	// This is less accurate but better than nothing
	for _, schema := range i.meta.Schemas {
		for _, table := range schema.Tables {
			for _, col := range table.Columns {
				if i.dialect.NormalizeIdentifier(col.Name) == i.dialect.NormalizeIdentifier(columnName) {
					return &core.InspectField{
						Name:   columnName,
						Table:  table.Name,
						Schema: schema.Name,
					}
				}
			}
		}
	}

	return nil
}

// extractWhereFields extracts column references and embedded subqueries from a select_core's WHERE.
func (i *Inspector) extractWhereFields(selectCore sqlite.ISelect_coreContext, relationRefs []core.RelationRef) ([]core.InspectField, []core.InspectStatement) {
	whereExpr := selectCore.GetWhereExpr()
	if whereExpr == nil {
		return nil, nil
	}
	return i.extractWhereFieldsFromExpr(whereExpr, relationRefs)
}

// extractWhereFieldsFromExpr extracts column references and embedded subqueries from a WHERE expression.
func (i *Inspector) extractWhereFieldsFromExpr(whereExpr sqlite.IExprContext, relationRefs []core.RelationRef) ([]core.InspectField, []core.InspectStatement) {
	if whereExpr == nil {
		return nil, nil
	}

	listener := &whereColumnExtractorListener{
		BaseSQLiteParserListener: &sqlite.BaseSQLiteParserListener{},
		inspector:                i,
		relationRefs:             relationRefs,
		fields:                   []core.InspectField{},
		seenFields:               make(map[string]bool),
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, whereExpr)

	subqueries := i.extractEmbeddedSubqueries(whereExpr)
	return listener.fields, subqueries
}

// extractEmbeddedSubqueries walks an AST node and returns InspectStatements for every
// select_stmt found inside expression trees. Only direct subqueries are collected;
// nested ones are captured recursively inside each collected subquery's own inspection.
func (i *Inspector) extractEmbeddedSubqueries(ctx antlr.ParseTree) []core.InspectStatement {
	listener := &embeddedSubqueryListener{
		BaseSQLiteParserListener: &sqlite.BaseSQLiteParserListener{},
		inspector:                i,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, ctx)
	return listener.results
}

type embeddedSubqueryListener struct {
	*sqlite.BaseSQLiteParserListener
	inspector     *Inspector
	results       []core.InspectStatement
	subqueryDepth int
}

func (l *embeddedSubqueryListener) EnterSelect_stmt(ctx *sqlite.Select_stmtContext) {
	if l.subqueryDepth == 0 {
		if sub := l.inspector.inspectSelect(ctx); sub != nil && len(sub.Tables) > 0 {
			l.results = append(l.results, *sub)
		}
	}
	l.subqueryDepth++
}

func (l *embeddedSubqueryListener) ExitSelect_stmt(_ *sqlite.Select_stmtContext) {
	l.subqueryDepth--
}

// extractSelectListSubqueries returns InspectStatements for scalar subqueries in the SELECT projection.
func (i *Inspector) extractSelectListSubqueries(selectCore sqlite.ISelect_coreContext) []core.InspectStatement {
	var subqueries []core.InspectStatement
	for _, resultCol := range selectCore.AllResult_column() {
		if expr := resultCol.Expr(); expr != nil {
			subqueries = append(subqueries, i.extractEmbeddedSubqueries(expr)...)
		}
	}
	return subqueries
}

// whereColumnExtractorListener extracts column references from WHERE expressions.
// It does not descend into subqueries, those are handled separately as InspectStatements.
type whereColumnExtractorListener struct {
	*sqlite.BaseSQLiteParserListener
	inspector     *Inspector
	relationRefs  []core.RelationRef
	fields        []core.InspectField
	seenFields    map[string]bool
	subqueryDepth int
}

func (l *whereColumnExtractorListener) EnterSelect_stmt(_ *sqlite.Select_stmtContext) {
	l.subqueryDepth++
}

func (l *whereColumnExtractorListener) ExitSelect_stmt(_ *sqlite.Select_stmtContext) {
	l.subqueryDepth--
}

func (l *whereColumnExtractorListener) EnterExpr(ctx *sqlite.ExprContext) {
	if ctx == nil || l.subqueryDepth > 0 {
		return
	}

	if colName := ctx.Column_name(); colName != nil {
		if anyName := colName.Any_name(); anyName != nil {
			columnName := l.inspector.dialect.NormalizeIdentifier(anyName.GetText())
			if columnName == "" {
				return
			}

			normalizedCol := columnName
			var tablePrefix string

			if ctx.Table_name() != nil && len(ctx.AllDOT()) > 0 {
				tablePrefix = l.inspector.dialect.NormalizeIdentifier(ctx.Table_name().Any_name().GetText())
			}

			var resolvedField *core.InspectField
			if tablePrefix != "" {
				for _, ref := range l.relationRefs {
					refKey := ref.Alias
					if refKey == "" {
						refKey = ref.Table
					}
					if l.inspector.dialect.NormalizeIdentifier(refKey) == tablePrefix {
						tableCols := core.GetColumnsForTableAsColumns(l.inspector.meta, ref.Schema, ref.Table, l.inspector.dialect)
						for _, col := range tableCols {
							if l.inspector.dialect.NormalizeIdentifier(col.Name) == normalizedCol {
								resolvedField = &core.InspectField{
									Name:   normalizedCol,
									Table:  ref.Table,
									Schema: ref.Schema,
								}
								break
							}
						}
						if resolvedField != nil {
							break
						}
					}
				}
			} else {
				for _, ref := range l.relationRefs {
					tableCols := core.GetColumnsForTableAsColumns(l.inspector.meta, ref.Schema, ref.Table, l.inspector.dialect)
					for _, col := range tableCols {
						if l.inspector.dialect.NormalizeIdentifier(col.Name) == normalizedCol {
							resolvedField = &core.InspectField{
								Name:   normalizedCol,
								Table:  ref.Table,
								Schema: ref.Schema,
							}
							break
						}
					}
					if resolvedField != nil {
						break
					}
				}
			}

			if resolvedField != nil {
				key := resolvedField.Schema + "." + resolvedField.Table + "." + resolvedField.Name
				if !l.seenFields[key] {
					l.seenFields[key] = true
					l.fields = append(l.fields, *resolvedField)
				}
			}
		}
	}
}

// convertRelationRefs converts RelationRef to InspectTable, filtering out virtual tables
func (i *Inspector) convertRelationRefs(refs []core.RelationRef, virtualTables map[string]bool) []core.InspectTable {
	var tables []core.InspectTable
	seen := make(map[string]bool)

	for _, ref := range refs {
		// Skip empty refs
		if ref.Schema == "" && ref.Table == "" {
			continue
		}

		// Skip virtual tables (CTEs, subqueries)
		if virtualTables != nil && virtualTables[i.dialect.NormalizeIdentifier(ref.Table)] {
			continue
		}

		// Mark unknown tables with Schema="" so the permission checker can deny them.
		schema := ref.Schema
		if !core.TableExistsInMetadata(i.meta, ref.Schema, ref.Table, i.dialect) {
			schema = ""
		}

		key := schema + "." + ref.Table
		if seen[key] {
			continue
		}
		seen[key] = true

		table := core.InspectTable{
			Name:      ref.Table,
			Schema:    schema,
			StartLine: ref.Line,
			StartCol:  ref.Col,
			EndCol:    ref.EndCol,
		}
		if ref.Alias != "" {
			table.Alias = &ref.Alias
		}
		tables = append(tables, table)
	}

	return tables
}

// extractCTEsWithSubqueries extracts CTE definitions and inspects their bodies
func (i *Inspector) extractCTEsWithSubqueries(commonTableStmt sqlite.ICommon_table_stmtContext) ([]core.RelationRef, []core.InspectStatement) {
	if commonTableStmt == nil {
		return nil, nil
	}

	var ctes []core.RelationRef
	var subqueries []core.InspectStatement

	cteElements := commonTableStmt.AllCommon_table_expression()
	if len(cteElements) == 0 {
		return ctes, subqueries
	}

	for _, cteEl := range cteElements {
		if cteEl == nil {
			continue
		}

		// Get CTE name
		cteName := ""
		if tableName := cteEl.Table_name(); tableName != nil {
			cteName = i.dialect.NormalizeIdentifier(tableName.Any_name().GetText())
		}

		// Inspect the CTE body to get its InspectStatement and columns
		var cteColumns []core.Column
		if selectStmt := cteEl.Select_stmt(); selectStmt != nil {
			if subResult := i.inspectSelect(selectStmt); subResult != nil {
				subqueries = append(subqueries, *subResult)
				// Extract column names from the subquery's fields
				for _, field := range subResult.Fields {
					cteColumns = append(cteColumns, core.Column{
						Name: field.Name,
						Type: "unknown",
					})
				}
			}
		}

		// Add CTE with its columns
		ctes = append(ctes, core.RelationRef{
			Table:     cteName,
			Columns:   cteColumns,
			IsVirtual: true,
		})
	}

	return ctes, subqueries
}

// extractFromSubqueries extracts InspectStatements from subqueries in the FROM clause
func (i *Inspector) extractFromSubqueries(selectCore sqlite.ISelect_coreContext, cteToSubqueryMap map[string]*core.InspectStatement) []core.InspectStatement {
	// Walk the FROM clause to find subqueries
	// In SQLite, FROM can contain table_or_subquery or join_clause
	// We need to check both
	listener := &subqueryExtractorListener{
		inspector:  i,
		subqueries: []core.InspectStatement{},
	}

	antlr.ParseTreeWalkerDefault.Walk(listener, selectCore)

	return listener.subqueries
}

// subqueryExtractorListener extracts subqueries from FROM clause
type subqueryExtractorListener struct {
	*sqlite.BaseSQLiteParserListener
	inspector  *Inspector
	subqueries []core.InspectStatement
}

func (l *subqueryExtractorListener) EnterTable_or_subquery(ctx *sqlite.Table_or_subqueryContext) {
	if ctx == nil {
		return
	}

	// Check if this is a subquery
	if selectStmt := ctx.Select_stmt(); selectStmt != nil {
		if subResult := l.inspector.inspectSelect(selectStmt); subResult != nil {
			l.subqueries = append(l.subqueries, *subResult)
		}
	}
}
