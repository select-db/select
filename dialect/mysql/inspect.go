package mysql

import (
	"strings"

	core "github.com/selectDb/dialect/core"
	mysql "github.com/selectDb/dialect/mysql/parser"

	antlr "github.com/antlr4-go/antlr/v4"
)

// Inspector analyzes SQL statements and extracts structured information.
type Inspector struct {
	dialect  *Dialect
	meta     core.Metadata
	resolver core.Resolver
}

// NewInspector creates a new MySQL statement inspector.
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

// Inspect parses the SQL and returns one InspectStatement per top-level statement.
func (i *Inspector) Inspect(sql string) []core.InspectStatement {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return nil
	}
	// The MySQL grammar requires a trailing ';'. Add one if missing so single
	// statements without it still parse.
	if !strings.HasSuffix(trimmed, ";") {
		trimmed += ";"
	}

	lexer := i.dialect.CreateLexer(trimmed)
	tokenStream := antlr.NewCommonTokenStream(lexer, 0)
	parser := mysql.NewMySQLParser(tokenStream)
	parser.RemoveErrorListeners()
	lexer.RemoveErrorListeners()
	syntax := core.NewSyntaxErrors()
	parser.AddErrorListener(syntax)

	var queries []mysql.IQueryContext
	if root := parser.Script(); root != nil {
		queries = root.AllQuery()
	}
	if len(queries) == 0 {
		return []core.InspectStatement{core.UnknownStatement()}
	}

	results := make([]core.InspectStatement, 0, len(queries))
	for idx, q := range queries {
		if q == nil {
			continue
		}
		from, to := core.TokenSpan(tokenStream, queries, idx)
		syntax.Cover(q, from, to)

		simple := q.SimpleStatement()
		if simple == nil {
			// The grammar emits a trailing empty query for the ';' we append.
			continue
		}
		read := core.OrUnknown(i.inspectStatement(simple))

		// INTO OUTFILE writes the server's filesystem, which the four row
		// actions do not cover. The clause is read off the tokens rather than
		// the tree because the spellings that follow a locking clause raise a
		// syntax error here, and error recovery drops the tail with the node.
		read = core.SalvageOrUnknown(read, syntax, from, to)
		if writesAFile(tokenStream, from, to) || callsHostFunction(tokenStream, from, to) {
			read = core.NestUnderUnknown(read)
		}
		results = append(results, read)
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

// writesAFile reports whether the tokens between from and to name a file to
// write. Both spellings follow INTO, and DUMPFILE is non-reserved, so a column
// named dumpfile lexes as the keyword and only the INTO tells them apart.
func writesAFile(tokens *antlr.CommonTokenStream, from, to int) bool {
	all := tokens.GetAllTokens()
	to = core.Clamp(to, 0, len(all))
	from = core.Clamp(from, 0, to)
	for ti := from; ti < to; ti++ {
		switch all[ti].GetTokenType() {
		case mysql.MySQLLexerOUTFILE_SYMBOL, mysql.MySQLLexerDUMPFILE_SYMBOL:
			if core.PrecededBy(all, from, ti, "into") {
				return true
			}
		}
	}
	return false
}

// inspectStatement dispatches based on the SimpleStatement variant.
func (i *Inspector) inspectStatement(stmt mysql.ISimpleStatementContext) *core.InspectStatement {
	if stmt == nil {
		return nil
	}
	if sel := stmt.SelectStatement(); sel != nil {
		return i.inspectSelectStatement(sel)
	}
	if ins := stmt.InsertStatement(); ins != nil {
		return i.inspectInsert(ins)
	}
	if upd := stmt.UpdateStatement(); upd != nil {
		return i.inspectUpdate(upd)
	}
	if del := stmt.DeleteStatement(); del != nil {
		return i.inspectDelete(del)
	}
	if rep := stmt.ReplaceStatement(); rep != nil {
		return i.inspectReplace(rep)
	}
	if trunc := stmt.TruncateTableStatement(); trunc != nil {
		return i.inspectTruncate(trunc)
	}
	if drop := stmt.DropStatement(); drop != nil {
		return i.inspectDrop(drop)
	}
	if alt := stmt.AlterStatement(); alt != nil {
		return i.inspectAlter(alt)
	}
	if cr := stmt.CreateStatement(); cr != nil {
		return i.inspectCreate(cr)
	}
	return nil
}

// ============================================
// SELECT
// ============================================

// inspectSelectStatement handles MySQL's SelectStatement -> QueryExpression /
// QueryExpressionParens / SelectStatementWithInto.
func (i *Inspector) inspectSelectStatement(stmt mysql.ISelectStatementContext) *core.InspectStatement {
	if qe := stmt.QueryExpression(); qe != nil {
		return i.inspectQueryExpression(qe)
	}
	if qep := stmt.QueryExpressionParens(); qep != nil {
		return i.inspectQueryExpressionParens(qep)
	}
	if into := stmt.SelectStatementWithInto(); into != nil {
		return i.inspectSelectStatementWithInto(into)
	}
	return nil
}

func (i *Inspector) inspectSelectStatementWithInto(ctx mysql.ISelectStatementWithIntoContext) *core.InspectStatement {
	if ctx == nil {
		return nil
	}
	// Descend into any nested QueryExpression we can find on this node.
	var inner *core.InspectStatement
	for _, child := range ctx.GetChildren() {
		switch c := child.(type) {
		case mysql.IQueryExpressionContext:
			inner = i.inspectQueryExpression(c)
		case mysql.ISelectStatementWithIntoContext:
			inner = i.inspectSelectStatementWithInto(c)
		}
		if inner != nil {
			break
		}
	}
	return inner
}

func (i *Inspector) inspectQueryExpressionParens(ctx mysql.IQueryExpressionParensContext) *core.InspectStatement {
	if ctx == nil {
		return nil
	}
	for _, child := range ctx.GetChildren() {
		switch c := child.(type) {
		case mysql.IQueryExpressionContext:
			return i.inspectQueryExpression(c)
		case mysql.IQueryExpressionParensContext:
			return i.inspectQueryExpressionParens(c)
		}
	}
	return nil
}

func (i *Inspector) inspectQueryExpression(qe mysql.IQueryExpressionContext) *core.InspectStatement {
	if qe == nil {
		return nil
	}

	// CTEs defined at the statement level (shared across UNION branches).
	var ctes []core.RelationRef
	var cteSubqueries []core.InspectStatement
	if w := qe.WithClause(); w != nil {
		ctes, cteSubqueries = i.extractCTEsFromWithClause(w)
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

	// Walk the body (which contains UNION/INTERSECT/EXCEPT branches).
	body := qe.QueryExpressionBody()
	parens := qe.QueryExpressionParens()
	if body != nil {
		i.mergeBranchesIntoResult(body, qe, result, ctes, cteSubqueries, cteToSubqueryMap)
	} else if parens != nil {
		if inner := i.inspectQueryExpressionParens(parens); inner != nil {
			result.Tables = core.MergeInspectTables(result.Tables, inner.Tables)
			result.Fields = core.MergeInspectFields(result.Fields, inner.Fields)
			result.Where = core.MergeInspectFields(result.Where, inner.Where)
			result.Subqueries = append(result.Subqueries, inner.Subqueries...)
		}
	}

	tail := i.extractTailSubqueries(qe)
	i.resolver.DropCTETables(tail, ctes)
	result.Subqueries = append(result.Subqueries, tail...)

	// The branches read the tail against their own relations, which is what
	// resolves a name a derived table gave. A bare name is read again here,
	// against the tables the statement ended up reading, which is what
	// resolves one the derived table passed straight through.
	result.Where = core.MergeInspectFields(result.Where,
		i.tailClauseFields(qe, core.RelationRefsOf(result), core.Scope{}))

	result.Where = core.DistinctTestsProjection(
		core.DedupsRows(qe, compoundOperators, mysql.MySQLParserALL_SYMBOL),
		result.Where, result.Fields)

	return result
}

// extractTailSubqueries collects the subqueries in ORDER BY, which sits after
// every UNION branch rather than inside one. MySQL's LIMIT takes an integer or
// a placeholder, never a subquery, so there is no tail clause beyond it.
func (i *Inspector) extractTailSubqueries(qe mysql.IQueryExpressionContext) []core.InspectStatement {
	if qe == nil {
		return nil
	}
	order := qe.OrderClause()
	if order == nil {
		return nil
	}
	return core.AsFilter(i.extractEmbeddedSubqueries(order))
}

// mergeBranchesIntoResult walks every QueryPrimary inside a QueryExpressionBody
// (and into nested QueryExpressionParens) and merges each branch into result.
func (i *Inspector) mergeBranchesIntoResult(
	body mysql.IQueryExpressionBodyContext,
	tail mysql.IQueryExpressionContext,
	result *core.InspectStatement,
	ctes []core.RelationRef,
	cteSubqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) {
	for _, prim := range body.AllQueryPrimary() {
		branch := i.inspectQueryPrimary(prim, tail, ctes, cteSubqueries, cteToSubqueryMap)
		if branch == nil {
			continue
		}
		result.Tables = core.MergeInspectTables(result.Tables, branch.Tables)
		result.Fields = core.MergeInspectFields(result.Fields, branch.Fields)
		result.Where = core.MergeInspectFields(result.Where, branch.Where)
		result.Subqueries = append(result.Subqueries, branch.Subqueries...)
	}
	for _, parens := range body.AllQueryExpressionParens() {
		if inner := i.inspectQueryExpressionParens(parens); inner != nil {
			result.Tables = core.MergeInspectTables(result.Tables, inner.Tables)
			result.Fields = core.MergeInspectFields(result.Fields, inner.Fields)
			result.Where = core.MergeInspectFields(result.Where, inner.Where)
			result.Subqueries = append(result.Subqueries, inner.Subqueries...)
		}
	}
}

// inspectQueryPrimary handles a single QueryPrimary (a QuerySpecification, a
// TableValueConstructor, or an ExplicitTable).
func (i *Inspector) inspectQueryPrimary(
	prim mysql.IQueryPrimaryContext,
	tail mysql.IQueryExpressionContext,
	ctes []core.RelationRef,
	cteSubqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) *core.InspectStatement {
	if prim == nil {
		return nil
	}
	// TABLE t1 is SELECT * FROM t1, and it reaches none of the target-list path.
	if explicit := prim.ExplicitTable(); explicit != nil {
		return i.inspectTableShorthand(explicit.TableRef())
	}
	spec := prim.QuerySpecification()
	if spec == nil {
		return nil
	}

	relationRefs, subqueryColumns := i.extractRelationRefs(spec)
	fromSubqueries := i.extractFromSubqueries(core.TreeOrNil(spec.FromClause()))

	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}
	tables := i.resolver.Tables(relationRefs, scope)
	for _, sub := range cteSubqueries {
		tables = core.MergeInspectTables(tables, sub.Tables)
	}
	for _, sub := range fromSubqueries {
		tables = core.MergeInspectTables(tables, sub.Tables)
	}

	allSubqueries := append([]core.InspectStatement{}, cteSubqueries...)
	allSubqueries = append(allSubqueries, fromSubqueries...)

	fields := i.extractSelectFieldsWithResolution(spec, relationRefs, ctes, subqueryColumns, allSubqueries, cteToSubqueryMap)

	where, whereSubqueries := i.extractWhereFields(spec, relationRefs, scope)
	where = core.MergeInspectFields(where, i.branchClauseFields(spec, relationRefs, scope, fields))
	where = core.MergeInspectFields(where, i.joinFields(core.TreeOrNil(spec.FromClause()), relationRefs, scope))
	where = core.MergeInspectFields(where, i.tailClauseFields(tail, relationRefs, scope))
	where = i.resolver.ThroughVirtual(where, relationRefs, scope, allSubqueries)
	selectSubqueries := i.extractSelectListSubqueries(spec)

	subqueries := append([]core.InspectStatement{}, fromSubqueries...)
	subqueries = append(subqueries, whereSubqueries...)
	subqueries = append(subqueries, selectSubqueries...)
	subqueries = append(subqueries, i.extractBranchClauseSubqueries(spec)...)
	i.resolver.DropCTETables(subqueries, ctes)

	return &core.InspectStatement{
		Operation:  core.InspectOpSelect,
		Tables:     tables,
		Fields:     fields,
		Where:      where,
		Subqueries: subqueries,
	}
}

// testedFields are the columns a clause names to choose, group or order rows
// rather than to return them, collected exactly as a WHERE's are. The listener
// does not descend into subqueries, which are collected in their own right.
func (i *Inspector) testedFields(tree antlr.ParseTree, refs []core.RelationRef, scope core.Scope) []core.InspectField {
	if tree == nil {
		return nil
	}
	listener := &whereColumnListener{
		BaseMySQLParserListener: &mysql.BaseMySQLParserListener{},
		inspector:               i,
		relationRefs:            refs,
		scope:                   scope,
		seen:                    make(map[string]bool),
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, tree)
	return listener.fields
}

// joinFields are the columns a join pairs rows on, wherever the join is: the
// FROM list of a select, or the relations a multi-table UPDATE or DELETE
// names. ON takes an expression, USING gives bare column names that belong to
// every relation carrying them, and NATURAL names nothing at all.
func (i *Inspector) joinFields(tree antlr.Tree, refs []core.RelationRef, scope core.Scope) []core.InspectField {
	if tree == nil {
		return nil
	}
	var fields []core.InspectField
	for _, join := range core.CollectNodes[mysql.IJoinedTableContext](tree) {
		fields = core.MergeInspectFields(fields, i.testedFields(core.TreeOrNil(join.Expr()), refs, scope))
		if list := join.IdentifierListWithParentheses(); list != nil && list.IdentifierList() != nil {
			names := make([]string, 0, len(list.IdentifierList().AllIdentifier()))
			for _, identifier := range list.IdentifierList().AllIdentifier() {
				names = append(names, i.dialect.NormalizeIdentifier(identifier.GetText()))
			}
			fields = core.MergeInspectFields(fields, i.resolver.NamedColumns(names, refs, scope))
		}
		if join.NaturalJoinType() != nil {
			fields = core.MergeInspectFields(fields, i.resolver.SharedColumns(refs, scope))
		}
	}
	return fields
}

// branchClauseFields are the columns the clauses of one branch name without
// returning: GROUP BY, HAVING and a named window.
func (i *Inspector) branchClauseFields(spec mysql.IQuerySpecificationContext, refs []core.RelationRef, scope core.Scope, projection []core.InspectField) []core.InspectField {
	if spec == nil {
		return nil
	}
	var fields []core.InspectField
	for _, clause := range []antlr.ParseTree{
		core.TreeOrNil(spec.GroupByClause()),
		core.TreeOrNil(spec.HavingClause()),
		core.TreeOrNil(spec.WindowClause()),
	} {
		fields = core.MergeInspectFields(fields, i.testedFields(clause, refs, scope))
	}
	fields = core.MergeInspectFields(fields, i.overAndFilterFields(spec, refs, scope))
	return core.DistinctTestsProjection(isDistinct(spec), fields, projection)
}

// overAndFilterFields are the columns an OVER or a FILTER names where the
// clause is written inline on a result column rather than as a WINDOW clause
// of its own. Both order or choose the rows an aggregate counts, so what they
// name is tested even where the column itself is never returned. MySQL has no
// FILTER clause.
func (i *Inspector) overAndFilterFields(tree antlr.Tree, refs []core.RelationRef, scope core.Scope) []core.InspectField {
	var fields []core.InspectField
	for _, over := range core.CollectNodes[mysql.IWindowingClauseContext](tree) {
		fields = core.MergeInspectFields(fields, i.testedFields(over, refs, scope))
	}
	return fields
}

// isDistinct reports whether a query specification carries SELECT DISTINCT.
// MySQL puts it among the select options rather than in a clause of its own.
func isDistinct(spec mysql.IQuerySpecificationContext) bool {
	for _, option := range spec.AllSelectOption() {
		if option == nil {
			continue
		}
		if specOption := option.QuerySpecOption(); specOption != nil && specOption.DISTINCT_SYMBOL() != nil {
			return true
		}
	}
	return false
}

// compoundOperators are the set operators whose plain form collapses duplicate
// rows.
var compoundOperators = []int{
	mysql.MySQLParserUNION_SYMBOL,
	mysql.MySQLParserINTERSECT_SYMBOL,
	mysql.MySQLParserEXCEPT_SYMBOL,
}

// tailClauseFields are the columns ORDER BY names. It sits after every branch
// of a compound select rather than inside one.
func (i *Inspector) tailClauseFields(qe mysql.IQueryExpressionContext, refs []core.RelationRef, scope core.Scope) []core.InspectField {
	if qe == nil {
		return nil
	}
	return i.testedFields(core.TreeOrNil(qe.OrderClause()), refs, scope)
}

// extractBranchClauseSubqueries collects the subqueries in the clauses of a
// branch that are neither the FROM list, the select list nor the WHERE. All
// three take an expression, and the server runs a subquery in each.
func (i *Inspector) extractBranchClauseSubqueries(spec mysql.IQuerySpecificationContext) []core.InspectStatement {
	if spec == nil {
		return nil
	}
	var subqueries []core.InspectStatement
	if group := spec.GroupByClause(); group != nil {
		subqueries = append(subqueries, i.extractEmbeddedSubqueries(group)...)
	}
	if having := spec.HavingClause(); having != nil {
		subqueries = append(subqueries, i.extractEmbeddedSubqueries(having)...)
	}
	// A window named here rather than written inline: the select-list walk
	// reaches OVER (...), not WINDOW w AS (...).
	if window := spec.WindowClause(); window != nil {
		subqueries = append(subqueries, i.extractEmbeddedSubqueries(window)...)
	}
	return core.AsFilter(subqueries)
}

// inspectTableShorthand analyzes TABLE t1, which is SELECT * FROM t1.
func (i *Inspector) inspectTableShorthand(ref mysql.ITableRefContext) *core.InspectStatement {
	unknown := core.UnknownStatement()
	schema, table := i.resolveTableRef(ref)
	if table == "" {
		return &unknown
	}
	// The columns are the statement, as they are for the SELECT * it stands
	// for. Without them the see check has no field to find and hides nothing.
	// No column means no such table, which resolves to no schema and is refused.
	fields := core.TableFields(i.meta, schema, table, i.dialect)
	if len(fields) == 0 {
		schema = ""
	}
	return &core.InspectStatement{
		Operation: core.InspectOpSelect,
		Tables:    []core.InspectTable{{Name: table, Schema: schema}},
		Fields:    fields,
	}
}

// ============================================
// INSERT
// ============================================

func (i *Inspector) inspectInsert(stmt mysql.IInsertStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpInsert}

	schema, tableName := i.resolveTableRef(stmt.TableRef())
	if tableName == "" {
		return nil
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	// Column list (from Fields).
	var explicitCols []string
	if fc := stmt.InsertFromConstructor(); fc != nil {
		if f := fc.Fields(); f != nil {
			explicitCols = i.collectInsertFields(f)
		}
	}
	if iqe := stmt.InsertQueryExpression(); iqe != nil {
		if f := iqe.Fields(); f != nil && len(explicitCols) == 0 {
			explicitCols = i.collectInsertFields(f)
		}
	}

	if len(explicitCols) > 0 {
		for _, name := range explicitCols {
			result.Fields = append(result.Fields, core.InspectField{
				Name:   name,
				Table:  tableName,
				Schema: schema,
			})
		}
	} else if stmt.SET_SYMBOL() != nil {
		if ul := stmt.UpdateList(); ul != nil {
			for _, el := range ul.AllUpdateElement() {
				if name := i.columnRefName(el.ColumnRef()); name != "" {
					result.Fields = append(result.Fields, core.InspectField{
						Name:   name,
						Table:  tableName,
						Schema: schema,
					})
				}
			}
		}
	} else {
		// No column list: expand to all columns from metadata.
		result.Fields = core.TableFields(i.meta, schema, tableName, i.dialect)
	}

	// INSERT … SELECT: source SELECT becomes a subquery.
	if iqe := stmt.InsertQueryExpression(); iqe != nil {
		if qop := iqe.QueryExpressionOrParens(); qop != nil {
			if sub := i.inspectQueryExpressionOrParens(qop); sub != nil && len(sub.Tables) > 0 {
				result.Subqueries = append(result.Subqueries, *sub)
			}
		}
	}
	// VALUES with embedded subqueries.
	if fc := stmt.InsertFromConstructor(); fc != nil {
		result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(fc)...)
	}
	// ON DUPLICATE KEY UPDATE: subqueries in RHS of SET expressions. The
	// clause also rewrites the row it conflicts with, so the row that was
	// there does not survive and insert alone is not the right it needs.
	if iul := stmt.InsertUpdateList(); iul != nil {
		result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(iul)...)
		core.AlsoPerforms(result, core.InspectOpUpdate,
			i.updateListFields(iul.UpdateList(), schema, tableName))
	}

	return result
}

// inspectReplace handles REPLACE statements. REPLACE shares its grammar shape
// with INSERT, so it goes through the same field and subquery extraction. It
// deletes whatever row it conflicts with before inserting, which is a right of
// its own rather than part of the write.
func (i *Inspector) inspectReplace(stmt mysql.IReplaceStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpInsert}

	schema, tableName := i.resolveTableRef(stmt.TableRef())
	if tableName == "" {
		return nil
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	var explicitCols []string
	if fc := stmt.InsertFromConstructor(); fc != nil {
		if f := fc.Fields(); f != nil {
			explicitCols = i.collectInsertFields(f)
		}
	}
	if iqe := stmt.InsertQueryExpression(); iqe != nil {
		if f := iqe.Fields(); f != nil && len(explicitCols) == 0 {
			explicitCols = i.collectInsertFields(f)
		}
	}

	if len(explicitCols) > 0 {
		for _, name := range explicitCols {
			result.Fields = append(result.Fields, core.InspectField{
				Name: name, Table: tableName, Schema: schema,
			})
		}
	} else if stmt.SET_SYMBOL() != nil {
		if ul := stmt.UpdateList(); ul != nil {
			for _, el := range ul.AllUpdateElement() {
				if name := i.columnRefName(el.ColumnRef()); name != "" {
					result.Fields = append(result.Fields, core.InspectField{
						Name: name, Table: tableName, Schema: schema,
					})
				}
			}
		}
	} else {
		result.Fields = core.TableFields(i.meta, schema, tableName, i.dialect)
	}

	if iqe := stmt.InsertQueryExpression(); iqe != nil {
		if qop := iqe.QueryExpressionOrParens(); qop != nil {
			if sub := i.inspectQueryExpressionOrParens(qop); sub != nil && len(sub.Tables) > 0 {
				result.Subqueries = append(result.Subqueries, *sub)
			}
		}
	}
	if fc := stmt.InsertFromConstructor(); fc != nil {
		result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(fc)...)
	}
	core.AlsoPerforms(result, core.InspectOpDelete, nil)
	return result
}

// updateListFields are the columns a SET list writes.
func (i *Inspector) updateListFields(
	list mysql.IUpdateListContext,
	schema, table string,
) []core.InspectField {
	if list == nil {
		return nil
	}
	var fields []core.InspectField
	for _, el := range list.AllUpdateElement() {
		if name := i.columnRefName(el.ColumnRef()); name != "" {
			fields = append(fields, core.InspectField{Name: name, Table: table, Schema: schema})
		}
	}
	return fields
}

// collectInsertFields collects the column names listed in (col1, col2, …).
func (i *Inspector) collectInsertFields(f mysql.IFieldsContext) []string {
	var names []string
	for _, ii := range f.AllInsertIdentifier() {
		if cr := ii.ColumnRef(); cr != nil {
			if name := i.columnRefName(cr); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func (i *Inspector) inspectQueryExpressionOrParens(ctx mysql.IQueryExpressionOrParensContext) *core.InspectStatement {
	if ctx == nil {
		return nil
	}
	for _, child := range ctx.GetChildren() {
		switch c := child.(type) {
		case mysql.IQueryExpressionContext:
			return i.inspectQueryExpression(c)
		case mysql.IQueryExpressionParensContext:
			return i.inspectQueryExpressionParens(c)
		}
	}
	return nil
}

// ============================================
// UPDATE
// ============================================

func (i *Inspector) inspectUpdate(stmt mysql.IUpdateStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpUpdate}

	// A WITH clause on a DML statement is the read it is on a SELECT, and the
	// write does not cover the rows it reads.
	ctes, cteBodies := i.extractCTEsFromWithClause(stmt.WithClause())
	result.Subqueries = append(result.Subqueries, cteBodies...)

	relationRefs, subqueryColumns := i.extractRelationRefsFromTableRefList(stmt.TableReferenceList())
	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns}
	fromSubqueries := i.extractFromSubqueries(core.TreeOrNil(stmt.TableReferenceList()))
	result.Subqueries = append(result.Subqueries, fromSubqueries...)

	result.Tables = i.resolver.Tables(relationRefs, scope)
	if len(result.Tables) == 0 {
		return nil
	}

	// First real table is the primary target. SET columns without table prefix attach to it.
	var targetSchema, targetTable string
	for _, ref := range relationRefs {
		if ref.Schema != "" {
			targetSchema = ref.Schema
			targetTable = ref.Table
			break
		}
	}

	if ul := stmt.UpdateList(); ul != nil {
		for _, el := range ul.AllUpdateElement() {
			colName := i.columnRefName(el.ColumnRef())
			if colName == "" {
				continue
			}
			schema, table := i.columnRefQualifierTable(el.ColumnRef(), relationRefs)
			if table == "" {
				table = targetTable
				schema = targetSchema
			}
			result.Fields = append(result.Fields, core.InspectField{
				Name:   colName,
				Table:  table,
				Schema: schema,
			})
			if expr := el.Expr(); expr != nil {
				result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(expr)...)
			}
		}
	}

	if wc := stmt.WhereClause(); wc != nil {
		if expr := wc.Expr(); expr != nil {
			where, subs := i.extractWhereFieldsFromExpr(expr, relationRefs, scope)
			result.Where = where
			result.Subqueries = append(result.Subqueries, subs...)
		}
	}
	result.Where = core.MergeInspectFields(result.Where,
		i.joinFields(core.TreeOrNil(stmt.TableReferenceList()), relationRefs, scope))

	// A multi-table UPDATE writes the tables its SET list names and reads the
	// rest. Leaving them in Tables asks for update on a table the statement
	// only joins against, which is a right it does not need and not the one
	// the read does.
	written, read := core.SplitWrittenTables(result.Tables, result.Fields)
	if len(written) > 0 {
		result.Tables = written
		core.AlsoReads(result, read)
	}
	return result
}

// ============================================
// DELETE
// ============================================

func (i *Inspector) inspectDelete(stmt mysql.IDeleteStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpDelete}

	// A WITH clause on a DML statement is the read it is on a SELECT, and the
	// write does not cover the rows it reads.
	ctes, cteBodies := i.extractCTEsFromWithClause(stmt.WithClause())
	result.Subqueries = append(result.Subqueries, cteBodies...)

	// sourceRefs covers every relation in scope for column resolution and the
	// WHERE clause; targetRefs are the tables actually being deleted from.
	var sourceRefs []core.RelationRef
	var targetRefs []core.RelationRef

	// Multi-table form (DELETE FROM list ... or DELETE alias_list FROM list).
	var subqueryColumns map[string][]core.Column
	if list := stmt.TableReferenceList(); list != nil {
		refs, columns := i.extractRelationRefsFromTableRefList(list)
		sourceRefs, subqueryColumns = refs, columns
		// A derived table in the list is a read of its own, and nothing else
		// in a DELETE reaches the tables behind it.
		result.Subqueries = append(result.Subqueries, i.extractFromSubqueries(list)...)
	}

	// Single-table form: DELETE FROM tbl [alias].
	if tr := stmt.TableRef(); tr != nil && len(sourceRefs) == 0 {
		schema, table := i.resolveTableRef(tr)
		if table != "" {
			ref := core.RelationRef{Schema: schema, Table: table}
			if ta := stmt.TableAlias(); ta != nil {
				ref.Alias = i.dialect.NormalizeIdentifier(ta.Identifier().GetText())
			}
			sourceRefs = append(sourceRefs, ref)
		}
	}
	// DELETE FROM tbl USING list_of_tables, tbl is the only target; USING
	// list became sourceRefs above. Make sure tbl is in sourceRefs too.
	if tr := stmt.TableRef(); tr != nil && len(sourceRefs) > 0 && stmt.USING_SYMBOL() != nil {
		schema, table := i.resolveTableRef(tr)
		if table != "" {
			targetRefs = append(targetRefs, core.RelationRef{Schema: schema, Table: table})
		}
	}

	// DELETE alias_list FROM list, resolve each alias against sourceRefs.
	if ar := stmt.TableAliasRefList(); ar != nil {
		for _, tw := range ar.AllTableRefWithWildcard() {
			text := strings.TrimSuffix(tw.GetText(), ".*")
			schema, name := splitQualifiedName(i.dialect, text, core.GetDefaultSchema(i.meta))
			if name == "" {
				continue
			}
			resolved := i.resolveDeleteTarget(name, schema, sourceRefs)
			targetRefs = append(targetRefs, resolved)
		}
	}

	// If neither USING nor an alias list was present, every source ref is also
	// a target (single-table form, or `DELETE FROM list_of_tables` is illegal
	// in MySQL anyway).
	if len(targetRefs) == 0 {
		targetRefs = sourceRefs
	}

	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns}
	result.Tables = i.resolver.Tables(targetRefs, scope)
	if len(result.Tables) == 0 {
		return nil
	}

	if wc := stmt.WhereClause(); wc != nil {
		if expr := wc.Expr(); expr != nil {
			where, subs := i.extractWhereFieldsFromExpr(expr, sourceRefs, scope)
			result.Where = where
			result.Subqueries = append(result.Subqueries, subs...)
		}
	}
	result.Where = core.MergeInspectFields(result.Where,
		i.joinFields(core.TreeOrNil(stmt.TableReferenceList()), sourceRefs, scope))

	// The relations a multi-table DELETE joins against without deleting from
	// are read, and nothing else in the statement reaches them.
	core.AlsoReads(result, core.TablesExcept(i.resolver.Tables(sourceRefs, scope), result.Tables))
	return result
}

// resolveDeleteTarget maps a target name (which may be a table name or an
// alias) to the matching relation in sourceRefs. If no match is found we
// fall back to the literal name with an empty schema so the permission
// checker can deny the unknown table.
func (i *Inspector) resolveDeleteTarget(name, schema string, sourceRefs []core.RelationRef) core.RelationRef {
	norm := i.dialect.NormalizeIdentifier(name)
	for _, ref := range sourceRefs {
		key := ref.Alias
		if key == "" {
			key = ref.Table
		}
		if i.dialect.NormalizeIdentifier(key) == norm {
			return ref
		}
	}
	return core.RelationRef{Schema: schema, Table: norm}
}

// ============================================
// TRUNCATE / DROP / ALTER / CREATE
// ============================================

func (i *Inspector) inspectTruncate(stmt mysql.ITruncateTableStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpTruncate}
	schema, table := i.resolveTableRef(stmt.TableRef())
	if table != "" {
		result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
	}
	return result
}

func (i *Inspector) inspectDrop(stmt mysql.IDropStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpDrop}
	if dt := stmt.DropTable(); dt != nil {
		if list := dt.TableRefList(); list != nil {
			for _, tr := range list.AllTableRef() {
				schema, table := i.resolveTableRef(tr)
				if table != "" {
					result.Tables = append(result.Tables, core.InspectTable{Name: table, Schema: schema})
				}
			}
		}
	}
	return result
}

func (i *Inspector) inspectAlter(stmt mysql.IAlterStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpAlter}
	if at := stmt.AlterTable(); at != nil {
		schema, table := i.resolveTableRef(at.TableRef())
		if table != "" {
			result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
		}
	}
	return result
}

func (i *Inspector) inspectCreate(stmt mysql.ICreateStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpCreate}
	if ct := stmt.CreateTable(); ct != nil {
		if tn := ct.TableName(); tn != nil {
			schema, table := i.resolveTableName(tn)
			if table != "" {
				result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
			}
		}
		if as := ct.DuplicateAsQueryExpression(); as != nil {
			result.Subqueries = i.sourceQuery(as.QueryExpressionOrParens())
		}
	}
	if cv := stmt.CreateView(); cv != nil {
		if vn := cv.ViewName(); vn != nil {
			schema, view := i.resolveViewName(vn)
			if view != "" {
				result.Tables = []core.InspectTable{{Name: view, Schema: schema}}
			}
		}
		if tail := cv.ViewTail(); tail != nil {
			if vs := tail.ViewSelect(); vs != nil {
				result.Subqueries = i.sourceQuery(vs.QueryExpressionOrParens())
			}
		}
	}
	return result
}

// sourceQuery is the query a CREATE TABLE ... AS or a CREATE VIEW is filled from, as its own
// statement: creating it needs manage, reading it still needs select.
func (i *Inspector) sourceQuery(source mysql.IQueryExpressionOrParensContext) []core.InspectStatement {
	if source == nil {
		return nil
	}
	return []core.InspectStatement{core.OrUnknown(i.inspectQueryExpressionOrParens(source))}
}

// ============================================
// HELPERS: name resolution
// ============================================

// resolveTableRef extracts (schema, table) from a TableRef. Schema falls back
// to the configured default when the name is unqualified.
func (i *Inspector) resolveTableRef(tr mysql.ITableRefContext) (schema, table string) {
	if tr == nil {
		return "", ""
	}
	return splitQualifiedName(i.dialect, tr.GetText(), core.GetDefaultSchema(i.meta))
}

// resolveViewName mirrors resolveTableRef for ViewName nodes.
func (i *Inspector) resolveViewName(vn mysql.IViewNameContext) (schema, view string) {
	if vn == nil {
		return "", ""
	}
	return splitQualifiedName(i.dialect, vn.GetText(), core.GetDefaultSchema(i.meta))
}

// resolveTableName mirrors resolveTableRef for TableName nodes (used by CREATE TABLE).
func (i *Inspector) resolveTableName(tn mysql.ITableNameContext) (schema, table string) {
	if tn == nil {
		return "", ""
	}
	return splitQualifiedName(i.dialect, tn.GetText(), core.GetDefaultSchema(i.meta))
}

// splitQualifiedName parses "db.table" or "table" and applies the default schema.
// Backtick-quoted segments are normalized.
func splitQualifiedName(d *Dialect, raw, defaultSchema string) (schema, table string) {
	schema, table, _ = splitQualifiedNameParts(d, raw, defaultSchema)
	return schema, table
}

// splitQualifiedNameParts also reports whether raw carried the schema. The
// caller cannot tell from schema alone, which holds the default for a bare name.
func splitQualifiedNameParts(d *Dialect, raw, defaultSchema string) (schema, table string, qualified bool) {
	parts := splitDotted(raw)
	switch len(parts) {
	case 0:
		return "", "", false
	case 1:
		return defaultSchema, d.NormalizeIdentifier(parts[0]), false
	default:
		return d.NormalizeIdentifier(parts[0]), d.NormalizeIdentifier(parts[len(parts)-1]), true
	}
}

// splitDotted splits "a.b" while honouring backtick-quoted segments containing dots.
func splitDotted(s string) []string {
	var parts []string
	var buf strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '`' {
			buf.WriteByte(c)
			inQuote = !inQuote
			continue
		}
		if c == '.' && !inQuote {
			parts = append(parts, buf.String())
			buf.Reset()
			continue
		}
		buf.WriteByte(c)
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}

// columnRefName returns the unqualified column name from a ColumnRef.
func (i *Inspector) columnRefName(cr mysql.IColumnRefContext) string {
	if cr == nil {
		return ""
	}
	fi := cr.FieldIdentifier()
	if fi == nil {
		return ""
	}
	// FieldIdentifier is either a QualifiedIdentifier or a DotIdentifier.
	if qi := fi.QualifiedIdentifier(); qi != nil {
		parts := splitDotted(qi.GetText())
		if di := fi.DotIdentifier(); di != nil {
			// schema.table.column form: QualifiedIdentifier holds schema.table,
			// DotIdentifier holds .column.
			parts = append(parts, strings.TrimPrefix(di.GetText(), "."))
		}
		if len(parts) == 0 {
			return ""
		}
		return i.dialect.NormalizeIdentifier(parts[len(parts)-1])
	}
	if di := fi.DotIdentifier(); di != nil {
		return i.dialect.NormalizeIdentifier(strings.TrimPrefix(di.GetText(), "."))
	}
	return ""
}

// columnRefQualifierTable returns (schema, table) for a qualified column ref
// like "t.c" or "s.t.c". Empty strings when unqualified.
func (i *Inspector) columnRefQualifierTable(cr mysql.IColumnRefContext, refs []core.RelationRef) (schema, table string) {
	if cr == nil {
		return "", ""
	}
	fi := cr.FieldIdentifier()
	if fi == nil {
		return "", ""
	}
	var parts []string
	if qi := fi.QualifiedIdentifier(); qi != nil {
		parts = splitDotted(qi.GetText())
		if di := fi.DotIdentifier(); di != nil {
			parts = append(parts, strings.TrimPrefix(di.GetText(), "."))
		}
	} else if di := fi.DotIdentifier(); di != nil {
		parts = []string{strings.TrimPrefix(di.GetText(), ".")}
	}
	if len(parts) < 2 {
		return "", ""
	}
	// schema.table.col
	if len(parts) == 3 {
		return i.dialect.NormalizeIdentifier(parts[0]), i.dialect.NormalizeIdentifier(parts[1])
	}
	// table.col, resolve to a known relation.
	prefix := i.dialect.NormalizeIdentifier(parts[0])
	for _, r := range refs {
		key := r.Alias
		if key == "" {
			key = r.Table
		}
		if i.dialect.NormalizeIdentifier(key) == prefix {
			return r.Schema, r.Table
		}
	}
	return "", prefix
}

// ============================================
// RELATION REF EXTRACTION (FROM clause)
// ============================================

// extractRelationRefs walks a QuerySpecification's FROM clause and returns the
// physical refs plus a map of subquery-alias -> inferred columns.
func (i *Inspector) extractRelationRefs(spec mysql.IQuerySpecificationContext) ([]core.RelationRef, map[string][]core.Column) {
	fc := spec.FromClause()
	if fc == nil {
		return nil, nil
	}
	list := fc.TableReferenceList()
	if list == nil {
		return nil, nil
	}
	return i.extractRelationRefsFromTableRefList(list)
}

func (i *Inspector) extractRelationRefsFromTableRefList(list mysql.ITableReferenceListContext) ([]core.RelationRef, map[string][]core.Column) {
	// Error recovery leaves no list at all, and walking a nil tree panics,
	// which fails the request rather than refusing the statement.
	if list == nil {
		return nil, nil
	}
	effectiveSchema := i.meta.CurrentSchema
	if effectiveSchema == "" {
		effectiveSchema = i.meta.DefaultSchema
	}
	listener := &relationRefListener{
		BaseMySQLParserListener: &mysql.BaseMySQLParserListener{},
		dialect:                 i.dialect,
		meta:                    i.meta,
		defaultSchema:           effectiveSchema,
		inspector:               i,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, list)

	subqueryColumns := make(map[string][]core.Column)
	for _, vt := range listener.vtabs {
		if len(vt.Columns) > 0 {
			subqueryColumns[vt.Table] = vt.Columns
		}
	}
	return listener.refs, subqueryColumns
}

// relationRefListener collects physical and derived (subquery-alias) refs
// from a TableReferenceList subtree.
type relationRefListener struct {
	*mysql.BaseMySQLParserListener
	dialect       *Dialect
	meta          core.Metadata
	defaultSchema string
	inspector     *Inspector

	refs           []core.RelationRef
	vtabs          []core.RelationRef
	subqueryDepth  int
	derivedCounter int
}

// Don't descend into FROM subqueries; their tables belong to their own scope.
func (l *relationRefListener) EnterSubquery(_ *mysql.SubqueryContext) {
	l.subqueryDepth++
}
func (l *relationRefListener) ExitSubquery(_ *mysql.SubqueryContext) {
	l.subqueryDepth--
}

func (l *relationRefListener) EnterSingleTable(ctx *mysql.SingleTableContext) {
	if l.subqueryDepth > 0 {
		return
	}
	tr := ctx.TableRef()
	if tr == nil {
		return
	}
	schema, table, qualified := splitQualifiedNameParts(l.dialect, tr.GetText(), l.defaultSchema)
	if table == "" {
		return
	}
	ref := core.RelationRef{
		Schema:        schema,
		Table:         table,
		Qualified:     qualified,
		ScopeStartPos: -1,
		ScopeEndPos:   -1,
	}
	if ta := ctx.TableAlias(); ta != nil {
		ref.Alias = l.dialect.NormalizeIdentifier(ta.Identifier().GetText())
	}
	if tok := tr.GetStart(); tok != nil {
		ref.Line = tok.GetLine()
		ref.Col = tok.GetColumn()
		ref.EndCol = tok.GetColumn() + len(tr.GetText())
	}
	l.refs = append(l.refs, ref)
}

func (l *relationRefListener) EnterDerivedTable(ctx *mysql.DerivedTableContext) {
	if l.subqueryDepth > 0 {
		return
	}

	alias := ""
	if ta := ctx.TableAlias(); ta != nil {
		alias = l.dialect.NormalizeIdentifier(ta.Identifier().GetText())
	}
	if alias == "" {
		l.derivedCounter++
		alias = "_subquery_" + string(rune('a'+l.derivedCounter-1))
	}

	// Infer columns from the subquery body.
	var cols []core.Column
	if cir := ctx.ColumnInternalRefList(); cir != nil {
		// Explicit column list: each ColumnInternalRef carries an Identifier.
		for _, child := range cir.GetChildren() {
			if c, ok := child.(mysql.IColumnInternalRefContext); ok {
				cols = append(cols, core.Column{
					Name: l.dialect.NormalizeIdentifier(c.Identifier().GetText()),
					Type: "unknown",
				})
			}
		}
	} else if sq := ctx.Subquery(); sq != nil {
		// Inspect the subquery and use its fields as the column list.
		if sub := l.inspector.inspectSubquery(sq); sub != nil {
			for _, f := range sub.Fields {
				cols = append(cols, core.Column{Name: f.Name, Type: "unknown"})
			}
		}
	}

	l.vtabs = append(l.vtabs, core.RelationRef{
		Table:     alias,
		Columns:   cols,
		IsVirtual: true,
	})
	l.refs = append(l.refs, core.RelationRef{
		Table:         alias,
		ScopeStartPos: -1,
		ScopeEndPos:   -1,
	})
}

// inspectSubquery inspects a MySQL Subquery node and returns its inspect
// statement (so callers can read out Fields).
func (i *Inspector) inspectSubquery(sq mysql.ISubqueryContext) *core.InspectStatement {
	if sq == nil {
		return nil
	}
	if qep := sq.QueryExpressionParens(); qep != nil {
		return i.inspectQueryExpressionParens(qep)
	}
	return nil
}

// ============================================
// SELECT FIELDS
// ============================================

func (i *Inspector) extractSelectFieldsWithResolution(
	spec mysql.IQuerySpecificationContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	subqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	fields := i.extractSelectFields(spec, relationRefs, ctes, subqueryColumns, cteToSubqueryMap)
	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}
	return i.resolver.ThroughVirtual(fields, relationRefs, scope, subqueries)
}

func (i *Inspector) extractSelectFields(
	spec mysql.IQuerySpecificationContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}

	// MySQL's QuerySpecification has either "SELECT *" (with MULT_OPERATOR on
	// the SelectItemList) or a list of SelectItems.
	il := spec.SelectItemList()
	if il == nil {
		return nil
	}
	if il.MULT_OPERATOR() != nil && len(il.AllSelectItem()) == 0 {
		return i.resolver.Star(relationRefs, scope)
	}

	var fields []core.InspectField
	// Account for the leading "*, col1, col2" case: SELECT *, c1, c2.
	if il.MULT_OPERATOR() != nil {
		fields = append(fields, i.resolver.Star(relationRefs, scope)...)
	}
	for _, item := range il.AllSelectItem() {
		fields = append(fields, i.processSelectItem(item, relationRefs, ctes, subqueryColumns, cteToSubqueryMap)...)
	}
	return fields
}

func (i *Inspector) processSelectItem(
	item mysql.ISelectItemContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	if item == nil {
		return nil
	}
	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}
	// table.*
	if tw := item.TableWild(); tw != nil {
		ids := tw.AllIdentifier()
		if len(ids) == 0 {
			return nil
		}
		prefix := i.dialect.NormalizeIdentifier(ids[len(ids)-1].GetText())
		return i.resolver.QualifiedStar(prefix, relationRefs, scope)
	}
	// Expression (column ref or computed).
	expr := item.Expr()
	if expr == nil {
		return nil
	}
	exprFields := i.extractFieldsFromExpr(expr, relationRefs, ctes, subqueryColumns, cteToSubqueryMap)
	if len(exprFields) == 1 {
		if alias := item.SelectAlias(); alias != nil {
			normalized := i.aliasName(alias)
			if normalized != "" {
				exprFields[0].Alias = &normalized
			}
		}
	}
	return exprFields
}

func (i *Inspector) aliasName(sa mysql.ISelectAliasContext) string {
	if sa == nil {
		return ""
	}
	if id := sa.Identifier(); id != nil {
		return i.dialect.NormalizeIdentifier(id.GetText())
	}
	if ts := sa.TextStringLiteral(); ts != nil {
		text := ts.GetText()
		text = strings.Trim(text, "'\"")
		return i.dialect.NormalizeIdentifier(text)
	}
	return ""
}

// ============================================
// EXPRESSION FIELD EXTRACTION
// ============================================

type exprColumnRef struct {
	name        string
	tablePrefix string
	schema      string
	line        int
	col         int
	endCol      int
}

func (i *Inspector) extractFieldsFromExpr(
	expr antlr.ParseTree,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	if expr == nil {
		return nil
	}
	listener := &exprColumnListener{
		BaseMySQLParserListener: &mysql.BaseMySQLParserListener{},
		inspector:               i,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, expr)

	var fields []core.InspectField
	for _, ref := range listener.columns {
		field := i.resolveColumn(ref, relationRefs, ctes, subqueryColumns, cteToSubqueryMap)
		if field != nil {
			field.StartLine = ref.line
			field.StartCol = ref.col
			field.EndCol = ref.endCol
			fields = append(fields, *field)
		}
	}
	return fields
}

type exprColumnListener struct {
	*mysql.BaseMySQLParserListener
	inspector     *Inspector
	columns       []exprColumnRef
	subqueryDepth int
}

func (l *exprColumnListener) EnterSubquery(_ *mysql.SubqueryContext) {
	l.subqueryDepth++
}
func (l *exprColumnListener) ExitSubquery(_ *mysql.SubqueryContext) {
	l.subqueryDepth--
}

func (l *exprColumnListener) EnterColumnRef(ctx *mysql.ColumnRefContext) {
	if l.subqueryDepth > 0 || ctx == nil {
		return
	}
	cr := l.inspector.columnRefParts(ctx)
	if cr.name == "" {
		return
	}
	l.columns = append(l.columns, cr)
}

// columnRefParts decomposes a ColumnRef into name, tablePrefix, schema, and
// source position.
func (i *Inspector) columnRefParts(ctx mysql.IColumnRefContext) exprColumnRef {
	var ref exprColumnRef
	fi := ctx.FieldIdentifier()
	if fi == nil {
		return ref
	}
	var parts []string
	if qi := fi.QualifiedIdentifier(); qi != nil {
		parts = splitDotted(qi.GetText())
		if di := fi.DotIdentifier(); di != nil {
			parts = append(parts, strings.TrimPrefix(di.GetText(), "."))
		}
	} else if di := fi.DotIdentifier(); di != nil {
		parts = []string{strings.TrimPrefix(di.GetText(), ".")}
	}
	if len(parts) == 0 {
		return ref
	}
	ref.name = i.dialect.NormalizeIdentifier(parts[len(parts)-1])
	if len(parts) >= 2 {
		ref.tablePrefix = i.dialect.NormalizeIdentifier(parts[len(parts)-2])
	}
	if len(parts) >= 3 {
		ref.schema = i.dialect.NormalizeIdentifier(parts[len(parts)-3])
	}
	if tok := ctx.GetStart(); tok != nil {
		ref.line = tok.GetLine()
		ref.col = tok.GetColumn()
		ref.endCol = tok.GetColumn() + len(ctx.GetText())
	}
	return ref
}

// resolveColumn maps a column reference to its source table using the relation
// refs in scope. Falls back to a single known table when ambiguous columns
// can't be resolved via metadata.
func (i *Inspector) resolveColumn(
	col exprColumnRef,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) *core.InspectField {
	scope := core.Scope{CTEs: ctes, Subqueries: subqueryColumns, CTEResults: cteToSubqueryMap}
	return i.resolver.SelectColumn(col.name, col.tablePrefix, nil, relationRefs, scope)
}

// ============================================
// WHERE
// ============================================

func (i *Inspector) extractWhereFields(spec mysql.IQuerySpecificationContext, refs []core.RelationRef, scope core.Scope) ([]core.InspectField, []core.InspectStatement) {
	wc := spec.WhereClause()
	if wc == nil {
		return nil, nil
	}
	return i.extractWhereFieldsFromExpr(wc.Expr(), refs, scope)
}

func (i *Inspector) extractWhereFieldsFromExpr(expr mysql.IExprContext, refs []core.RelationRef, scope core.Scope) ([]core.InspectField, []core.InspectStatement) {
	if expr == nil {
		return nil, nil
	}
	listener := &whereColumnListener{
		BaseMySQLParserListener: &mysql.BaseMySQLParserListener{},
		inspector:               i,
		relationRefs:            refs,
		scope:                   scope,
		seen:                    make(map[string]bool),
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, expr)

	subs := core.AsFilter(i.extractEmbeddedSubqueries(expr))
	return listener.fields, subs
}

type whereColumnListener struct {
	*mysql.BaseMySQLParserListener
	inspector    *Inspector
	relationRefs []core.RelationRef
	// scope is what the statement declared, so a name a CTE or a derived
	// table returns resolves to the column behind it.
	scope         core.Scope
	fields        []core.InspectField
	seen          map[string]bool
	subqueryDepth int
}

func (l *whereColumnListener) EnterSubquery(_ *mysql.SubqueryContext) {
	l.subqueryDepth++
}
func (l *whereColumnListener) ExitSubquery(_ *mysql.SubqueryContext) {
	l.subqueryDepth--
}

func (l *whereColumnListener) EnterColumnRef(ctx *mysql.ColumnRefContext) {
	if l.subqueryDepth > 0 || ctx == nil {
		return
	}
	cr := l.inspector.columnRefParts(ctx)
	if cr.name == "" {
		return
	}
	var resolved *core.InspectField
	if cr.tablePrefix != "" {
		resolved = l.inspector.resolver.Column(cr.tablePrefix, cr.name, l.relationRefs)
	} else {
		resolved = l.inspector.resolver.UnqualifiedColumn(cr.name, l.relationRefs, l.scope)
		if resolved == nil && len(l.relationRefs) == 1 {
			ref := l.relationRefs[0]
			resolved = &core.InspectField{Name: cr.name, Table: ref.Table, Schema: ref.Schema}
		}
	}
	if resolved == nil {
		return
	}
	key := resolved.Schema + "." + resolved.Table + "." + resolved.Name
	if l.seen[key] {
		return
	}
	l.seen[key] = true
	l.fields = append(l.fields, *resolved)
}

// ============================================
// EMBEDDED SUBQUERIES
// ============================================

func (i *Inspector) extractEmbeddedSubqueries(node antlr.ParseTree) []core.InspectStatement {
	if node == nil {
		return nil
	}
	listener := &embeddedSubqueryListener{
		BaseMySQLParserListener: &mysql.BaseMySQLParserListener{},
		inspector:               i,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, node)
	return listener.results
}

type embeddedSubqueryListener struct {
	*mysql.BaseMySQLParserListener
	inspector     *Inspector
	results       []core.InspectStatement
	subqueryDepth int
}

func (l *embeddedSubqueryListener) EnterSubquery(ctx *mysql.SubqueryContext) {
	if l.subqueryDepth == 0 && ctx != nil {
		if sub := l.inspector.inspectSubquery(ctx); sub != nil && len(sub.Tables) > 0 {
			l.results = append(l.results, *sub)
		}
	}
	l.subqueryDepth++
}
func (l *embeddedSubqueryListener) ExitSubquery(_ *mysql.SubqueryContext) {
	l.subqueryDepth--
}

// extractSelectListSubqueries collects subqueries appearing in the SELECT
// projection (scalar subqueries).
func (i *Inspector) extractSelectListSubqueries(spec mysql.IQuerySpecificationContext) []core.InspectStatement {
	il := spec.SelectItemList()
	if il == nil {
		return nil
	}
	var out []core.InspectStatement
	for _, item := range il.AllSelectItem() {
		if expr := item.Expr(); expr != nil {
			out = append(out, i.extractEmbeddedSubqueries(expr)...)
		}
	}
	return out
}

// extractFromSubqueries collects InspectStatements for the derived tables in a
// FROM clause or in the table reference list an UPDATE or a DELETE reads.
func (i *Inspector) extractFromSubqueries(tree antlr.Tree) []core.InspectStatement {
	if tree == nil {
		return nil
	}
	listener := &fromSubqueryListener{
		BaseMySQLParserListener: &mysql.BaseMySQLParserListener{},
		inspector:               i,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, tree)
	return listener.results
}

type fromSubqueryListener struct {
	*mysql.BaseMySQLParserListener
	inspector *Inspector
	results   []core.InspectStatement
	depth     int
}

// EnterSubquery, only direct subqueries at depth 0 count; nested are
// handled inside their parent.
func (l *fromSubqueryListener) EnterSubquery(ctx *mysql.SubqueryContext) {
	if l.depth == 0 && ctx != nil {
		// Walk up to confirm we're inside a DerivedTable (FROM subquery), not
		// an expression-level subquery (those are picked up by extractSelectListSubqueries
		// and extractWhereFields).
		if isDerivedTableSubquery(ctx) {
			if sub := l.inspector.inspectSubquery(ctx); sub != nil && len(sub.Tables) > 0 {
				l.results = append(l.results, *sub)
			}
		}
	}
	l.depth++
}
func (l *fromSubqueryListener) ExitSubquery(_ *mysql.SubqueryContext) {
	l.depth--
}

func isDerivedTableSubquery(ctx *mysql.SubqueryContext) bool {
	parent := ctx.GetParent()
	for parent != nil {
		if _, ok := parent.(*mysql.DerivedTableContext); ok {
			return true
		}
		parent = parent.GetParent()
	}
	return false
}

// ============================================
// CTEs
// ============================================

func (i *Inspector) extractCTEsFromWithClause(w mysql.IWithClauseContext) ([]core.RelationRef, []core.InspectStatement) {
	if w == nil {
		return nil, nil
	}
	var ctes []core.RelationRef
	var subs []core.InspectStatement

	// Names before bodies: a body cannot be walked until the clause it may refer
	// to is known.
	elements := w.AllCommonTableExpression()
	names := make([]string, 0, len(elements))
	for _, cte := range elements {
		name := ""
		if cte != nil {
			if id := cte.Identifier(); id != nil {
				name = i.dialect.NormalizeIdentifier(id.GetText())
			}
		}
		names = append(names, name)
	}
	recursive := w.RECURSIVE_SYMBOL() != nil

	for idx, cte := range elements {
		if cte == nil {
			continue
		}
		name := names[idx]
		var cols []core.Column
		if sq := cte.Subquery(); sq != nil {
			if sub := i.inspectSubquery(sq); sub != nil {
				subs = append(subs, *sub)
				i.resolver.DropVirtual(subs[len(subs)-1:], core.CTEScope(names, idx, recursive))
				for _, f := range subs[len(subs)-1].Fields {
					cols = append(cols, core.Column{Name: f.Name, Type: "unknown"})
				}
			}
		}
		ctes = append(ctes, core.RelationRef{
			Table:     name,
			Columns:   cols,
			IsVirtual: true,
		})
	}
	return ctes, subs
}

// ============================================
// CONVERT REFS -> InspectTable
// ============================================

// resolve binds the shared resolution rules to this inspector's metadata.
