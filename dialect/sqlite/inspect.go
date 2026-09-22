package sqlite

import (
	"strings"

	core "github.com/selectDb/dialect/core"
	sqlite "github.com/selectDb/dialect/sqlite/parser"

	antlr "github.com/antlr4-go/antlr/v4"
)

// Inspector analyzes SQL statements and extracts structured information
type Inspector struct {
	dialect  *Dialect
	meta     core.Metadata
	resolver core.Resolver
}

// NewInspector creates a new SQLite statement inspector
func NewInspector(dialect *Dialect, meta core.Metadata) *Inspector {
	return &Inspector{
		dialect:  dialect,
		meta:     meta,
		resolver: core.Resolver{Meta: meta, Dialect: dialect},
	}
}

// Inspect implements core.SQLDialect.Inspect by delegating to a fresh Inspector.
func (d *Dialect) Inspect(meta core.Metadata, sql string) []core.InspectStatement {
	return NewInspector(d, meta).Inspect(sql)
}

// Inspect analyzes SQL and returns structured results for each statement
func (i *Inspector) Inspect(sql string) []core.InspectStatement {
	if strings.TrimSpace(sql) == "" {
		return nil
	}

	lexer := i.dialect.CreateLexer(sql)
	tokenStream := antlr.NewCommonTokenStream(lexer, 0)
	tokenStream.Fill() // pre-fill for compound-operator detection
	parser := sqlite.NewSQLiteParser(tokenStream)
	parser.RemoveErrorListeners()
	syntax := core.NewSyntaxErrors()
	parser.AddErrorListener(syntax)

	var stmtLists []sqlite.ISql_stmt_listContext
	if root := parser.Parse(); root != nil {
		stmtLists = root.AllSql_stmt_list()
	}
	if len(stmtLists) == 0 {
		return []core.InspectStatement{core.UnknownStatement()}
	}

	var results []core.InspectStatement
	idx := 0
	cursor := 0
	for idx < len(stmtLists) {
		// Collect consecutive stmt_lists connected by compound operators (UNION/INTERSECT/EXCEPT).
		// The SQLite grammar emits each UNION branch as a separate sql_stmt_list at the top level.
		first := idx
		group := []sqlite.ISql_stmt_listContext{stmtLists[idx]}
		dedups := false
		for idx+1 < len(stmtLists) && hasCompoundOperatorBetween(tokenStream, stmtLists[idx], stmtLists[idx+1]) {
			dedups = dedups || compoundDedupsBetween(tokenStream, stmtLists[idx], stmtLists[idx+1])
			idx++
			group = append(group, stmtLists[idx])
		}

		// A call that reaches the filesystem is not covered by the four row
		// actions, and neither is a statement the parser stumbled over that
		// named no table: a per-table check has nothing to ask about, so what
		// error recovery salvaged would run on a policy granting nothing. A
		// compound group is one statement, so its branches are read together; a
		// list of statements is read one at a time, so neither costs the rest
		// of the script its row actions.
		groupFrom, _ := core.TokenSpan(tokenStream, stmtLists, first)
		_, groupTo := core.TokenSpan(tokenStream, stmtLists, idx)
		// Error recovery can skip the tokens before a statement, which leaves
		// them belonging to nobody: "REVOKE SELECT ON t1 FROM bob" starts its
		// only statement at the SELECT, so the error on REVOKE falls outside
		// every span and the salvaged read looks like a statement of its own.
		// Every token belongs to the statement that follows it.
		groupFrom = core.Clamp(cursor, 0, groupFrom)
		cursor = groupTo
		syntax.Cover(stmtLists[idx], groupFrom, groupTo)

		if len(group) > 1 {
			read := core.OrUnknown(i.mergeCompoundSelectGroup(group, dedups))
			read = core.SalvageOrUnknown(read, syntax, groupFrom, groupTo)
			if callsHostFunction(tokenStream, groupFrom, groupTo) {
				read = core.NestUnderUnknown(read)
			}
			results = append(results, read)
			idx++
			continue
		}

		stmts := group[0].AllSql_stmt()
		for si := range stmts {
			read := core.OrUnknown(i.inspectStatement(stmts[si]))
			from, to := core.TokenSpan(tokenStream, stmts, si)
			if si == 0 {
				from = core.Clamp(groupFrom, 0, from)
			}
			to = core.Clamp(to, from, groupTo)
			read = core.SalvageOrUnknown(read, syntax, from, to)
			if callsHostFunction(tokenStream, from, to) {
				read = core.NestUnderUnknown(read)
			}
			results = append(results, read)
		}
		idx++
	}

	if syntax.Uncovered() {
		results = append(results, core.UnknownStatement())
	}

	// A subquery was inspected against its own FROM alone, so a name it takes
	// from the statement around it resolved to nothing there. The enclosing
	// relations are in scope here.
	i.resolver.ResolveCorrelated(results, nil)

	return results
}

// hasCompoundOperatorBetween reports whether UNION/INTERSECT/EXCEPT tokens appear between two parse-tree nodes.
// The compound operator is the last token of the first stmt_list, so we scan from stopIdx (inclusive).
func hasCompoundOperatorBetween(tokens *antlr.CommonTokenStream, a, b antlr.ParserRuleContext) bool {
	// Error recovery leaves a node without its bounding tokens, and reading one
	// off it panics, which fails the request rather than refusing the statement.
	if a == nil || b == nil || a.GetStop() == nil || b.GetStart() == nil {
		return false
	}
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

// compoundDedupsBetween reports whether the compound operator between two
// branches collapses duplicate rows, which every one of UNION, INTERSECT and
// EXCEPT does unless it is written with ALL.
func compoundDedupsBetween(tokens *antlr.CommonTokenStream, a, b antlr.ParserRuleContext) bool {
	if a == nil || b == nil || a.GetStop() == nil || b.GetStart() == nil {
		return false
	}
	allTokens := tokens.GetAllTokens()
	operator := false
	for ti := a.GetStop().GetTokenIndex(); ti < b.GetStart().GetTokenIndex() && ti < len(allTokens); ti++ {
		token := allTokens[ti]
		if token.GetChannel() != antlr.TokenDefaultChannel {
			continue
		}
		switch strings.ToUpper(token.GetText()) {
		case "UNION", "INTERSECT", "EXCEPT":
			operator = true
		case "ALL":
			if operator {
				return false
			}
		}
	}
	return operator
}

// mergeCompoundSelectGroup merges consecutive stmt_lists that are compound
// SELECT branches. dedups says the operator joining them collapses duplicate
// rows, which makes the row count a test on the values.
func (i *Inspector) mergeCompoundSelectGroup(group []sqlite.ISql_stmt_listContext, dedups bool) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpSelect}
	var last sqlite.ISelect_stmtContext
	for _, stmtList := range group {
		for _, stmt := range stmtList.AllSql_stmt() {
			if selectStmt := stmt.Select_stmt(); selectStmt != nil {
				branch := i.inspectSelect(selectStmt)
				if branch == nil {
					continue
				}
				last = selectStmt
				result.Tables = core.MergeInspectTables(result.Tables, branch.Tables)
				result.Fields = core.MergeInspectFields(result.Fields, branch.Fields)
				result.Where = core.MergeInspectFields(result.Where, branch.Where)
				result.Subqueries = append(result.Subqueries, branch.Subqueries...)
			}
		}
	}
	// The ORDER BY of a compound select parses onto its last branch, but it
	// orders the rows of every branch, so it is read again against all of the
	// relations the branches named.
	result.Where = core.MergeInspectFields(result.Where,
		i.tailClauseFields(last, core.RelationRefsOf(result), core.Scope{}))
	result.Where = core.DistinctTestsProjection(dedups, result.Where, result.Fields)
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
	if viewStmt := stmt.Create_view_stmt(); viewStmt != nil {
		return i.inspectCreateView(viewStmt)
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
		branch := i.inspectSelectCore(selectCore, selectStmt, ctes, cteSubqueries, cteToSubqueryMap)
		result.Tables = core.MergeInspectTables(result.Tables, branch.Tables)
		result.Fields = core.MergeInspectFields(result.Fields, branch.Fields)
		result.Where = core.MergeInspectFields(result.Where, branch.Where)
		result.Subqueries = append(result.Subqueries, branch.Subqueries...)
	}

	tail := i.extractTailSubqueries(selectStmt)
	i.resolver.DropCTETables(tail, ctes)
	result.Subqueries = append(result.Subqueries, tail...)

	// The branches read the tail against their own relations, which is what
	// resolves a name a derived table gave. A bare name is read again here,
	// against the tables the statement ended up reading, which is what
	// resolves one the derived table passed straight through.
	result.Where = core.MergeInspectFields(result.Where,
		i.tailClauseFields(selectStmt, core.RelationRefsOf(result), core.Scope{}))

	result.Where = core.DistinctTestsProjection(
		core.DedupsRows(selectStmt, compoundOperators, sqlite.SQLiteParserALL_),
		result.Where, result.Fields)

	return result
}

// compoundOperators are the set operators whose plain form collapses duplicate
// rows.
var compoundOperators = []int{
	sqlite.SQLiteParserUNION_,
	sqlite.SQLiteParserINTERSECT_,
	sqlite.SQLiteParserEXCEPT_,
}

// tailClauseFields are the columns ORDER BY and LIMIT name. They sit after
// every branch of a compound select rather than inside one.
func (i *Inspector) tailClauseFields(selectStmt sqlite.ISelect_stmtContext, refs []core.RelationRef, scope core.Scope) []core.InspectField {
	if selectStmt == nil {
		return nil
	}
	var fields []core.InspectField
	if order := selectStmt.Order_by_stmt(); order != nil {
		fields = core.MergeInspectFields(fields, i.testedFields(order, refs, scope))
	}
	if limit := selectStmt.Limit_stmt(); limit != nil {
		fields = core.MergeInspectFields(fields, i.testedFields(limit, refs, scope))
	}
	return fields
}

// testedFields are the columns a clause names to choose, group or order rows
// rather than to return them, collected exactly as a WHERE's are. The listener
// does not descend into subqueries, which are collected in their own right.
func (i *Inspector) testedFields(tree antlr.ParseTree, refs []core.RelationRef, scope core.Scope) []core.InspectField {
	if tree == nil {
		return nil
	}
	listener := &whereColumnExtractorListener{
		BaseSQLiteParserListener: &sqlite.BaseSQLiteParserListener{},
		inspector:                i,
		relationRefs:             refs,
		scope:                    scope,
		fields:                   []core.InspectField{},
		seenFields:               make(map[string]bool),
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, tree)
	return listener.fields
}

// joinFields are the columns a join pairs rows on, wherever the join is: the
// FROM list of a select, or the relations an UPDATE reads. ON names an
// expression, USING gives bare column names that belong to every relation
// carrying them, and NATURAL names nothing at all.
func (i *Inspector) joinFields(tree antlr.Tree, refs []core.RelationRef, scope core.Scope) []core.InspectField {
	if tree == nil {
		return nil
	}
	var fields []core.InspectField
	for _, constraint := range core.CollectNodes[sqlite.IJoin_constraintContext](tree) {
		columns := constraint.AllColumn_name()
		if len(columns) == 0 {
			fields = core.MergeInspectFields(fields, i.testedFields(constraint, refs, scope))
			continue
		}
		names := make([]string, 0, len(columns))
		for _, column := range columns {
			names = append(names, i.dialect.NormalizeIdentifier(column.GetText()))
		}
		fields = core.MergeInspectFields(fields, i.resolver.NamedColumns(names, refs, scope))
	}
	// The keyword is read off the tokens because the grammar prefers to read
	// the NATURAL in "t1 NATURAL JOIN t2" as an alias of t1, leaving the join
	// operator holding JOIN alone.
	for _, node := range core.CollectNodes[antlr.TerminalNode](tree) {
		if node.GetSymbol().GetTokenType() == sqlite.SQLiteParserNATURAL_ {
			fields = core.MergeInspectFields(fields, i.resolver.SharedColumns(refs, scope))
			break
		}
	}
	return fields
}

// branchClauseFields are the columns the clauses of one branch name without
// returning: GROUP BY, HAVING and a named window.
func (i *Inspector) branchClauseFields(selectCore sqlite.ISelect_coreContext, refs []core.RelationRef, scope core.Scope, projection []core.InspectField) []core.InspectField {
	if selectCore == nil {
		return nil
	}
	var fields []core.InspectField
	for _, group := range selectCore.GetGroupByExpr() {
		fields = core.MergeInspectFields(fields, i.testedFields(group, refs, scope))
	}
	if having := selectCore.GetHavingExpr(); having != nil {
		fields = core.MergeInspectFields(fields, i.testedFields(having, refs, scope))
	}
	for _, window := range selectCore.AllWindow_defn() {
		if window != nil {
			fields = core.MergeInspectFields(fields, i.testedFields(window, refs, scope))
		}
	}
	fields = core.MergeInspectFields(fields, i.overAndFilterFields(selectCore, refs, scope))
	return core.DistinctTestsProjection(selectCore.DISTINCT_() != nil, fields, projection)
}

// overAndFilterFields are the columns an OVER or a FILTER names where the
// clause is written inline on a result column rather than as a WINDOW clause
// of its own. Both order or choose the rows an aggregate counts, so what they
// name is tested even where the column itself is never returned.
func (i *Inspector) overAndFilterFields(tree antlr.Tree, refs []core.RelationRef, scope core.Scope) []core.InspectField {
	var fields []core.InspectField
	for _, over := range core.CollectNodes[sqlite.IOver_clauseContext](tree) {
		fields = core.MergeInspectFields(fields, i.testedFields(over, refs, scope))
	}
	for _, filter := range core.CollectNodes[sqlite.IFilter_clauseContext](tree) {
		fields = core.MergeInspectFields(fields, i.testedFields(filter, refs, scope))
	}
	return fields
}

// extractTailSubqueries collects the subqueries in the clauses that sit after
// every compound branch rather than inside one. ORDER BY, LIMIT and OFFSET each
// take an expression, and the server runs a subquery in all three. Limit_stmt
// carries LIMIT and OFFSET together.
func (i *Inspector) extractTailSubqueries(selectStmt sqlite.ISelect_stmtContext) []core.InspectStatement {
	if selectStmt == nil {
		return nil
	}
	var subqueries []core.InspectStatement
	if order := selectStmt.Order_by_stmt(); order != nil {
		subqueries = append(subqueries, i.extractEmbeddedSubqueries(order)...)
	}
	if limit := selectStmt.Limit_stmt(); limit != nil {
		subqueries = append(subqueries, i.extractEmbeddedSubqueries(limit)...)
	}
	return core.AsFilter(subqueries)
}

// inspectSelectCore processes a single select_core (one branch of a compound query).
func (i *Inspector) inspectSelectCore(
	selectCore sqlite.ISelect_coreContext,
	tail sqlite.ISelect_stmtContext,
	ctes []core.RelationRef,
	cteSubqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) core.InspectStatement {
	relationRefs, subqueryColumns := i.extractRelationRefs(selectCore)

	fromSubqueries := i.extractFromSubqueries(selectCore, cteToSubqueryMap)

	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}
	tables := i.resolver.Tables(relationRefs, scope)
	for _, subq := range fromSubqueries {
		tables = core.MergeInspectTables(tables, subq.Tables)
	}

	// The CTE bodies come first: extractSelectFieldsWithResolution walks this
	// slice by index, the CTEs against its head and the subquery aliases
	// against what follows. Handing it the subqueries alone maps a CTE name to
	// a subquery's columns and leaves the alias resolving to nothing.
	allSubqueries := make([]core.InspectStatement, 0, len(cteSubqueries)+len(fromSubqueries))
	allSubqueries = append(allSubqueries, cteSubqueries...)
	allSubqueries = append(allSubqueries, fromSubqueries...)
	fields := i.extractSelectFieldsWithResolution(selectCore, relationRefs, ctes, subqueryColumns, allSubqueries, cteToSubqueryMap)

	where, whereSubqueries := i.extractWhereFields(selectCore, relationRefs, scope)
	selectSubqueries := i.extractSelectListSubqueries(selectCore)

	subqueries := append(fromSubqueries, whereSubqueries...)
	subqueries = append(subqueries, selectSubqueries...)
	subqueries = append(subqueries, i.extractBranchClauseSubqueries(selectCore)...)
	i.resolver.DropCTETables(subqueries, ctes)

	tested := core.MergeInspectFields(where, i.branchClauseFields(selectCore, relationRefs, scope, fields))
	tested = core.MergeInspectFields(tested, i.joinFields(selectCore, relationRefs, scope))
	tested = core.MergeInspectFields(tested, i.tailClauseFields(tail, relationRefs, scope))

	return core.InspectStatement{
		Tables:     tables,
		Fields:     fields,
		Where:      i.resolver.ThroughVirtual(tested, relationRefs, scope, allSubqueries),
		Subqueries: subqueries,
	}
}

// extractBranchClauseSubqueries collects the subqueries in the clauses of a
// branch that are neither the FROM list, the result columns nor the WHERE. All
// three take an expression, and the server runs a subquery in each.
func (i *Inspector) extractBranchClauseSubqueries(selectCore sqlite.ISelect_coreContext) []core.InspectStatement {
	if selectCore == nil {
		return nil
	}
	var subqueries []core.InspectStatement
	for _, group := range selectCore.GetGroupByExpr() {
		if group != nil {
			subqueries = append(subqueries, i.extractEmbeddedSubqueries(group)...)
		}
	}
	if having := selectCore.GetHavingExpr(); having != nil {
		subqueries = append(subqueries, i.extractEmbeddedSubqueries(having)...)
	}
	// A window named here rather than written inline: the result-column walk
	// reaches OVER (...), not WINDOW w AS (...).
	for _, window := range selectCore.AllWindow_defn() {
		if window != nil {
			subqueries = append(subqueries, i.extractEmbeddedSubqueries(window)...)
		}
	}
	return core.AsFilter(subqueries)
}

// effectiveSchema is the schema an unqualified name resolves in. SQLite is the
// dialect that honours CurrentSchema, so the fallback lives in one place.
func (i *Inspector) effectiveSchema() string {
	if i.meta.CurrentSchema != "" {
		return i.meta.CurrentSchema
	}
	return i.meta.DefaultSchema
}

// targetAlias is the name an UPDATE or DELETE target goes by in the rest of
// the statement. Without it a column qualified with the alias resolves to no
// table, and a column that resolves to no table is checked against nothing.
func (i *Inspector) targetAlias(qtname sqlite.IQualified_table_nameContext) string {
	if qtname == nil || qtname.Alias() == nil {
		return ""
	}
	return i.dialect.NormalizeIdentifier(qtname.Alias().GetText())
}

// resolveQualifiedTableName extracts schema and table name from a qualified_table_name context.
func (i *Inspector) resolveQualifiedTableName(qtname sqlite.IQualified_table_nameContext) (schema, table string) {
	if qtname == nil {
		return "", ""
	}
	schema = i.effectiveSchema()
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
	schema = i.effectiveSchema()
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

	ctes, cteBodies := i.inspectWithClause(stmt.With_clause())
	result.Subqueries = append(result.Subqueries, cteBodies...)

	schema, tableName := i.resolveInsertTarget(stmt)
	if tableName == "" {
		return nil
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
		result.Fields = core.TableFields(i.meta, schema, tableName, i.dialect)
	}

	// INSERT ... SELECT: attach source as subquery when it has real tables.
	if selectStmt := stmt.Select_stmt(); selectStmt != nil {
		if sub := i.inspectSelect(selectStmt); sub != nil && len(sub.Tables) > 0 {
			result.Subqueries = append(result.Subqueries, *sub)
		}
	} else {
		// VALUES form: walk expressions for embedded subqueries.
		result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(stmt)...)
	}

	// An ON CONFLICT clause chooses which rows it updates and reads values into
	// them, both against the target table, so what it names is tested.
	result.Where = core.MergeInspectFields(result.Where,
		i.testedFields(core.TreeOrNil(stmt.Upsert_clause()),
			[]core.RelationRef{{Table: tableName, Schema: schema}}, core.Scope{}))

	// REPLACE, and its INSERT OR REPLACE spelling, delete whatever conflicts
	// before inserting, and an upsert rewrites it. Either way the row that was
	// there does not survive, so insert alone is not the right the statement
	// needs.
	if stmt.REPLACE_() != nil {
		core.AlsoPerforms(result, core.InspectOpDelete, nil)
	}
	if upsert := stmt.Upsert_clause(); upsert != nil && upsert.UPDATE_() != nil {
		core.AlsoPerforms(result, core.InspectOpUpdate,
			i.upsertSetFields(upsert, schema, tableName))
	}

	i.resolver.DropCTETables(result.Subqueries, ctes)

	i.addReturningFields(result, stmt.Returning_clause(), schema, tableName)

	return result
}

// upsertSetFields are the columns DO UPDATE writes. The clause names columns
// on both sides of SET, the conflict target before it and the assignments
// after, so the token index is what tells them apart.
func (i *Inspector) upsertSetFields(
	upsert sqlite.IUpsert_clauseContext,
	schema, table string,
) []core.InspectField {
	set := upsert.SET_()
	if set == nil {
		return nil
	}
	after := set.GetSymbol().GetTokenIndex()
	var fields []core.InspectField
	for _, name := range upsert.AllColumn_name() {
		start := name.GetStart()
		if start == nil || start.GetTokenIndex() < after {
			continue
		}
		fields = append(fields, core.InspectField{
			Name:   i.dialect.NormalizeIdentifier(name.GetText()),
			Table:  table,
			Schema: schema,
		})
	}
	return fields
}

// inspectUpdate analyzes an UPDATE statement.
func (i *Inspector) inspectUpdate(stmt sqlite.IUpdate_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpUpdate}

	schema, tableName := i.resolveQualifiedTableName(stmt.Qualified_table_name())
	if tableName == "" {
		return nil
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	ctes, cteBodies := i.inspectWithClause(stmt.With_clause())
	result.Subqueries = append(result.Subqueries, cteBodies...)
	reads, sourceRefs := i.readSources(stmt, ctes)
	result.Subqueries = append(result.Subqueries, reads...)

	// SET column names, AllColumn_name() returns the LHS of each assignment.
	for _, cn := range stmt.AllColumn_name() {
		name := i.dialect.NormalizeIdentifier(cn.Any_name().GetText())
		result.Fields = append(result.Fields, core.InspectField{
			Name:   name,
			Table:  tableName,
			Schema: schema,
		})
	}

	whereRefs := append([]core.RelationRef{
		{Table: tableName, Schema: schema, Alias: i.targetAlias(stmt.Qualified_table_name())},
	}, sourceRefs...)

	// RHS expressions: the columns they read and the subqueries they embed,
	// excluding the WHERE expr.
	allExprs := stmt.AllExpr()
	rhsExprs := allExprs
	if stmt.WHERE_() != nil && len(allExprs) > 0 {
		rhsExprs = allExprs[:len(allExprs)-1]
	}
	var stored []core.InspectField
	for _, expr := range rhsExprs {
		stored = core.MergeInspectFields(stored,
			i.testedFields(expr, whereRefs, core.Scope{CTEs: ctes}))
		result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(expr)...)
	}

	// WHERE clause: the last Expr() is the WHERE condition when WHERE_ token is present.
	if stmt.WHERE_() != nil {
		if len(allExprs) > 0 {
			whereExpr := allExprs[len(allExprs)-1]
			where, whereSubqueries := i.extractWhereFieldsFromExpr(whereExpr, whereRefs, core.Scope{CTEs: ctes})
			result.Where = where
			result.Subqueries = append(result.Subqueries, whereSubqueries...)
		}
	}

	result.Where = core.MergeInspectFields(result.Where,
		i.joinFields(stmt, whereRefs, core.Scope{CTEs: ctes}))
	// A column on the right of an assignment is read and its value stored, so
	// it belongs with what the statement reads without returning it.
	result.Where = core.MergeInspectFields(result.Where, stored)

	i.resolver.DropCTETables(result.Subqueries, ctes)

	i.addReturningFields(result, stmt.Returning_clause(), schema, tableName)

	return result
}

func (i *Inspector) inspectWithClause(with sqlite.IWith_clauseContext) ([]core.RelationRef, []core.InspectStatement) {
	if with == nil {
		return nil, nil
	}
	elements := with.AllCte_table_name()
	bodies := with.AllSelect_stmt()

	// Names before bodies: a body cannot be walked until the clause it may refer
	// to is known. This is the CTE clause an UPDATE carries; the one a SELECT
	// carries is extractCTEsWithSubqueries, and both need the same scope.
	names := make([]string, 0, len(elements))
	for _, element := range elements {
		name := ""
		if element.Table_name() != nil {
			name = i.dialect.NormalizeIdentifier(element.Table_name().GetText())
		}
		names = append(names, name)
	}
	recursive := with.RECURSIVE_() != nil

	ctes := make([]core.RelationRef, 0, len(elements))
	subqueries := make([]core.InspectStatement, 0, len(elements))
	for idx, element := range elements {
		if element.Table_name() == nil {
			continue
		}
		ctes = append(ctes, core.RelationRef{
			Table:     names[idx],
			IsVirtual: true,
		})
		if idx < len(bodies) {
			subqueries = append(subqueries, core.OrUnknown(i.inspectSelect(bodies[idx])))
			body := subqueries[len(subqueries)-1:]
			i.resolver.DropVirtual(body, core.CTEScope(names, idx, recursive))
		}
	}
	return ctes, subqueries
}

// readSources is the read an UPDATE ... FROM performs on the relations it joins
// against. Those rows are read, not written, so the update does not cover them.
// It also reports the relations, which the WHERE resolves names against.
func (i *Inspector) readSources(stmt sqlite.IUpdate_stmtContext, ctes []core.RelationRef) ([]core.InspectStatement, []core.RelationRef) {
	// A FROM list is either a comma list of relations or a join clause, and
	// only the relations under it are read; the target table is not.
	relations := make([]antlr.ParseTree, 0, len(stmt.AllTable_or_subquery())+1)
	for _, relation := range stmt.AllTable_or_subquery() {
		relations = append(relations, relation)
	}
	if join := stmt.Join_clause(); join != nil {
		relations = append(relations, join)
	}

	var refs []core.RelationRef
	var reads []core.InspectStatement
	subqueryColumns := make(map[string][]core.Column)

	for _, relation := range relations {
		relationRefs, columns := i.extractRelationRefs(relation)
		refs = append(refs, relationRefs...)
		for name, cols := range columns {
			subqueryColumns[name] = cols
		}
		reads = append(reads, i.extractFromSubqueries(relation, nil)...)
	}

	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns}
	if tables := i.resolver.Tables(refs, scope); len(tables) > 0 {
		reads = append(reads, core.InspectStatement{
			Operation: core.InspectOpSelect,
			Tables:    tables,
		})
	}
	return reads, refs
}

// inspectDelete analyzes a DELETE statement.
func (i *Inspector) inspectDelete(stmt sqlite.IDelete_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpDelete}

	ctes, cteBodies := i.inspectWithClause(stmt.With_clause())
	result.Subqueries = append(result.Subqueries, cteBodies...)

	schema, tableName := i.resolveQualifiedTableName(stmt.Qualified_table_name())
	if tableName == "" {
		return nil
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	if stmt.WHERE_() != nil && stmt.Expr() != nil {
		targetRef := []core.RelationRef{
			{Table: tableName, Schema: schema, Alias: i.targetAlias(stmt.Qualified_table_name())},
		}
		where, whereSubqueries := i.extractWhereFieldsFromExpr(stmt.Expr(), targetRef, core.Scope{})
		result.Where = where
		result.Subqueries = append(result.Subqueries, whereSubqueries...)
	}

	i.resolver.DropCTETables(result.Subqueries, ctes)

	i.addReturningFields(result, stmt.Returning_clause(), schema, tableName)

	return result
}

// inspectDrop analyzes a DROP TABLE/INDEX/VIEW/TRIGGER statement.
func (i *Inspector) inspectDrop(stmt sqlite.IDrop_stmtContext) *core.InspectStatement {
	// Only handle DROP TABLE; other object types don't need permission checks.
	if stmt.TABLE_() == nil {
		return nil
	}
	result := &core.InspectStatement{Operation: core.InspectOpDrop}
	schema := i.effectiveSchema()
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
	schema := i.effectiveSchema()
	if stmt.Schema_name() != nil {
		schema = i.dialect.NormalizeIdentifier(stmt.Schema_name().GetText())
	}
	if stmt.Table_name() != nil {
		table := i.dialect.NormalizeIdentifier(stmt.Table_name().Any_name().GetText())
		result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
	}
	result.Subqueries = i.sourceQuery(stmt.Select_stmt())
	return result
}

// inspectCreateView analyzes CREATE VIEW ... AS SELECT. Creating the view needs
// manage; the query behind it reads its own tables, which manage does not stand
// in for.
func (i *Inspector) inspectCreateView(stmt sqlite.ICreate_view_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpCreate}
	schema := i.effectiveSchema()
	if stmt.Schema_name() != nil {
		schema = i.dialect.NormalizeIdentifier(stmt.Schema_name().GetText())
	}
	if stmt.View_name() != nil {
		view := i.dialect.NormalizeIdentifier(stmt.View_name().GetText())
		result.Tables = []core.InspectTable{{Name: view, Schema: schema}}
	}
	result.Subqueries = i.sourceQuery(stmt.Select_stmt())
	return result
}

// sourceQuery is the query a CREATE TABLE ... AS or a CREATE VIEW is filled from, as its own
// statement: creating it needs manage, reading it still needs select.
func (i *Inspector) sourceQuery(selectStmt sqlite.ISelect_stmtContext) []core.InspectStatement {
	if selectStmt == nil {
		return nil
	}
	return []core.InspectStatement{core.OrUnknown(i.inspectSelect(selectStmt))}
}

// inspectAlterTable analyzes an ALTER TABLE statement.
func (i *Inspector) inspectAlterTable(stmt sqlite.IAlter_table_stmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpAlter}
	schema := i.effectiveSchema()
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

// extractRelationRefs extracts table references from any node that can hold
// them: a select_core, or the relation list of an UPDATE ... FROM.
func (i *Inspector) extractRelationRefs(tree antlr.ParseTree) ([]core.RelationRef, map[string][]core.Column) {
	// Use a listener similar to the dialect's relationRefListener
	listener := &relationRefExtractorListener{
		BaseSQLiteParserListener: &sqlite.BaseSQLiteParserListener{},
		refs:                     []core.RelationRef{},
		vtabs:                    []core.RelationRef{},
		defaultSchema:            i.effectiveSchema(),
		meta:                     i.meta,
		dialect:                  i.dialect,
		level:                    0,
		subqueryDepth:            0,
		depthStack:               []bool{},
		root:                     tree,
	}

	antlr.ParseTreeWalkerDefault.Walk(listener, tree)

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
	// root is the tree this listener was started on. A nested statement is
	// inspected on its own, but its context keeps the parent pointers of the
	// whole parse, so an ancestor walk has to stop here.
	root antlr.Tree
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
			ref.Qualified = true
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
				ref.Qualified = true
			}
			ref.Table = l.dialect.NormalizeIdentifier(tableName.Any_name().GetText())

			// Skip JOIN keywords that might be parsed as table names
			if l.isJoinKeyword(ref.Table) {
				continue
			}

			// Check if this is a CTE (virtual table) - CTEs have no schema. Only
			// an unqualified name can be one.
			if !ref.Qualified && l.isCTE(ref.Table) {
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
	// Walk up the parse tree to see if we're inside a Join_clause, stopping at
	// the tree being inspected: a join further out belongs to a statement this
	// listener is not walking, and EnterJoin_clause will not run for it.
	parent := ctx.GetParent()
	for parent != nil && parent != l.root {
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
	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}
	return i.resolver.ThroughVirtual(fields, relationRefs, scope, subqueries)
}

// extractSelectFields extracts fields from the SELECT clause
func (i *Inspector) extractSelectFields(
	selectCore sqlite.ISelect_coreContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}

	// Get result columns directly from select_core
	resultColumns := selectCore.AllResult_column()
	if len(resultColumns) == 0 {
		return nil
	}

	var fields []core.InspectField

	for _, resultCol := range resultColumns {
		// Handle SELECT * (case 1: just STAR)
		if resultCol.STAR() != nil && resultCol.Table_name() == nil {
			fields = append(fields, i.resolver.Star(relationRefs, scope)...)
			continue
		}

		// Handle table.* (case 2: table_name DOT STAR)
		if resultCol.Table_name() != nil && resultCol.DOT() != nil && resultCol.STAR() != nil {
			tableName := resultCol.Table_name().Any_name().GetText()
			normalizedTable := i.dialect.NormalizeIdentifier(tableName)
			fields = append(fields, i.resolver.QualifiedStar(normalizedTable, relationRefs, scope)...)
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
	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}
	return i.resolver.SelectColumn(columnName, tablePrefix, alias, relationRefs, scope)
}

// extractWhereFields extracts column references and embedded subqueries from a select_core's WHERE.
func (i *Inspector) extractWhereFields(selectCore sqlite.ISelect_coreContext, relationRefs []core.RelationRef, scope core.Scope) ([]core.InspectField, []core.InspectStatement) {
	whereExpr := selectCore.GetWhereExpr()
	if whereExpr == nil {
		return nil, nil
	}
	return i.extractWhereFieldsFromExpr(whereExpr, relationRefs, scope)
}

// extractWhereFieldsFromExpr extracts column references and embedded subqueries from a WHERE expression.
func (i *Inspector) extractWhereFieldsFromExpr(whereExpr sqlite.IExprContext, relationRefs []core.RelationRef, scope core.Scope) ([]core.InspectField, []core.InspectStatement) {
	if whereExpr == nil {
		return nil, nil
	}

	listener := &whereColumnExtractorListener{
		BaseSQLiteParserListener: &sqlite.BaseSQLiteParserListener{},
		inspector:                i,
		relationRefs:             relationRefs,
		scope:                    scope,
		fields:                   []core.InspectField{},
		seenFields:               make(map[string]bool),
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, whereExpr)

	subqueries := core.AsFilter(i.extractEmbeddedSubqueries(whereExpr))
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
	inspector    *Inspector
	relationRefs []core.RelationRef
	// scope is what the statement declared, so a name a CTE or a derived
	// table returns resolves to the column behind it.
	scope         core.Scope
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
				resolvedField = l.inspector.resolver.Column(tablePrefix, normalizedCol, l.relationRefs)
			} else {
				resolvedField = l.inspector.resolver.UnqualifiedColumn(normalizedCol, l.relationRefs, l.scope)
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

	// Names before bodies: a body cannot be walked until the clause it may refer
	// to is known.
	names := make([]string, 0, len(cteElements))
	for _, cteEl := range cteElements {
		name := ""
		if cteEl != nil {
			if tableName := cteEl.Table_name(); tableName != nil {
				name = i.dialect.NormalizeIdentifier(tableName.Any_name().GetText())
			}
		}
		names = append(names, name)
	}
	recursive := commonTableStmt.RECURSIVE_() != nil

	for idx, cteEl := range cteElements {
		if cteEl == nil {
			continue
		}

		cteName := names[idx]

		// Inspect the CTE body to get its InspectStatement and columns
		var cteColumns []core.Column
		if selectStmt := cteEl.Select_stmt(); selectStmt != nil {
			if subResult := i.inspectSelect(selectStmt); subResult != nil {
				subqueries = append(subqueries, *subResult)
				i.resolver.DropVirtual(subqueries[len(subqueries)-1:], core.CTEScope(names, idx, recursive))
				// Extract column names from the subquery's fields
				for _, field := range subqueries[len(subqueries)-1].Fields {
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
func (i *Inspector) extractFromSubqueries(tree antlr.ParseTree, cteToSubqueryMap map[string]*core.InspectStatement) []core.InspectStatement {
	listener := &subqueryExtractorListener{
		inspector:  i,
		subqueries: []core.InspectStatement{},
	}

	antlr.ParseTreeWalkerDefault.Walk(listener, tree)

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

// resolve binds the shared resolution rules to this inspector's metadata.
// addReturningFields records the columns a RETURNING clause hands back. They
// are read from the target table and reach the caller's rows, so a rule hiding
// one has to find it here as it would in a select.
func (i *Inspector) addReturningFields(
	result *core.InspectStatement,
	ret sqlite.IReturning_clauseContext,
	schema, table string,
) {
	if ret == nil {
		return
	}
	refs := []core.RelationRef{{Table: table, Schema: schema}}
	var returned []core.InspectField
	for _, column := range ret.AllResult_column() {
		if column.STAR() != nil {
			returned = core.MergeInspectFields(returned,
				core.TableFields(i.meta, schema, table, i.dialect))
			continue
		}
		if expr := column.Expr(); expr != nil {
			returned = core.MergeInspectFields(returned,
				i.extractFieldsFromExpr(expr, refs, nil, nil, nil))
		}
	}
	// RETURNING hands rows back, and rows handed back are a read whatever
	// wrote them. They stay out of the write's own fields, which are the
	// columns it stores.
	core.AlsoPerforms(result, core.InspectOpSelect, returned)
}
