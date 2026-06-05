package mysql

import (
	"strings"

	core "github.com/selectDb/dialect/core"
	mysql "github.com/selectDb/dialect/mysql/parser"

	antlr "github.com/antlr4-go/antlr/v4"
)

// Inspector analyzes SQL statements and extracts structured information.
type Inspector struct {
	dialect *Dialect
	meta    core.Metadata
}

// NewInspector creates a new MySQL statement inspector.
func NewInspector(dialect *Dialect, meta core.Metadata) *Inspector {
	return &Inspector{dialect: dialect, meta: meta}
}

// Inspect implements core.SQLDialect.Inspect by delegating to a fresh Inspector.
func (d *Dialect) Inspect(meta core.Metadata, sql string) []core.InspectStatement {
	return NewInspector(d, meta).Inspect(sql)
}

func (i *Inspector) normalizeEquals(a, b string) bool {
	return i.dialect.NormalizeIdentifier(a) == i.dialect.NormalizeIdentifier(b)
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

	root := parser.Script()
	if root == nil {
		return nil
	}

	queries := root.AllQuery()
	if len(queries) == 0 {
		return nil
	}

	var results []core.InspectStatement
	for _, q := range queries {
		if q == nil {
			continue
		}
		simple := q.SimpleStatement()
		if simple == nil {
			continue
		}
		if r := i.inspectStatement(simple); r != nil {
			results = append(results, *r)
		}
	}
	return results
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
		// SELECT … INTO; inspect the inner query expression for permission purposes.
		// SelectStatementWithInto wraps a QueryExpression with INTO clauses we don't track.
		return i.inspectSelectStatementWithInto(into)
	}
	return nil
}

func (i *Inspector) inspectSelectStatementWithInto(ctx mysql.ISelectStatementWithIntoContext) *core.InspectStatement {
	if ctx == nil {
		return nil
	}
	// Descend into any nested QueryExpression we can find on this node.
	for _, child := range ctx.GetChildren() {
		switch c := child.(type) {
		case mysql.IQueryExpressionContext:
			return i.inspectQueryExpression(c)
		case mysql.ISelectStatementWithIntoContext:
			return i.inspectSelectStatementWithInto(c)
		}
	}
	return nil
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
		i.mergeBranchesIntoResult(body, result, ctes, cteSubqueries, cteToSubqueryMap)
	} else if parens != nil {
		if inner := i.inspectQueryExpressionParens(parens); inner != nil {
			result.Tables = core.MergeInspectTables(result.Tables, inner.Tables)
			result.Fields = core.MergeInspectFields(result.Fields, inner.Fields)
			result.Where = core.MergeInspectFields(result.Where, inner.Where)
			result.Subqueries = append(result.Subqueries, inner.Subqueries...)
		}
	}
	return result
}

// mergeBranchesIntoResult walks every QueryPrimary inside a QueryExpressionBody
// (and into nested QueryExpressionParens) and merges each branch into result.
func (i *Inspector) mergeBranchesIntoResult(
	body mysql.IQueryExpressionBodyContext,
	result *core.InspectStatement,
	ctes []core.RelationRef,
	cteSubqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) {
	for _, prim := range body.AllQueryPrimary() {
		branch := i.inspectQueryPrimary(prim, ctes, cteSubqueries, cteToSubqueryMap)
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
	ctes []core.RelationRef,
	cteSubqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) *core.InspectStatement {
	if prim == nil {
		return nil
	}
	spec := prim.QuerySpecification()
	if spec == nil {
		return nil
	}

	relationRefs, subqueryColumns := i.extractRelationRefs(spec)
	fromSubqueries := i.extractFromSubqueries(spec)

	virtualTables := make(map[string]bool)
	for _, cte := range ctes {
		virtualTables[i.dialect.NormalizeIdentifier(cte.Table)] = true
	}
	for name := range subqueryColumns {
		virtualTables[i.dialect.NormalizeIdentifier(name)] = true
	}

	tables := i.convertRelationRefs(relationRefs, virtualTables)
	for _, sub := range cteSubqueries {
		tables = core.MergeInspectTables(tables, sub.Tables)
	}
	for _, sub := range fromSubqueries {
		tables = core.MergeInspectTables(tables, sub.Tables)
	}

	allSubqueries := append([]core.InspectStatement{}, cteSubqueries...)
	allSubqueries = append(allSubqueries, fromSubqueries...)

	fields := i.extractSelectFieldsWithResolution(spec, relationRefs, ctes, subqueryColumns, allSubqueries, cteToSubqueryMap)

	where, whereSubqueries := i.extractWhereFields(spec, relationRefs)
	selectSubqueries := i.extractSelectListSubqueries(spec)

	subqueries := append([]core.InspectStatement{}, fromSubqueries...)
	subqueries = append(subqueries, whereSubqueries...)
	subqueries = append(subqueries, selectSubqueries...)

	return &core.InspectStatement{
		Operation:  core.InspectOpSelect,
		Tables:     tables,
		Fields:     fields,
		Where:      where,
		Subqueries: subqueries,
	}
}

// ============================================
// INSERT
// ============================================

func (i *Inspector) inspectInsert(stmt mysql.IInsertStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpInsert}

	schema, tableName := i.resolveTableRef(stmt.TableRef())
	if tableName == "" {
		return result
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
		for _, col := range core.GetColumnsForTableAsColumns(i.meta, schema, tableName, i.dialect) {
			result.Fields = append(result.Fields, core.InspectField{
				Name:   col.Name,
				Table:  tableName,
				Schema: schema,
			})
		}
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
	// ON DUPLICATE KEY UPDATE: subqueries in RHS of SET expressions.
	if iul := stmt.InsertUpdateList(); iul != nil {
		result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(iul)...)
	}

	return result
}

// inspectReplace handles REPLACE statements. REPLACE shares its grammar shape
// with INSERT, so it goes through the same field/subquery extraction. We use
// InspectOpInsert because the permission semantics are identical (write).
func (i *Inspector) inspectReplace(stmt mysql.IReplaceStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpInsert}

	schema, tableName := i.resolveTableRef(stmt.TableRef())
	if tableName == "" {
		return result
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
		for _, col := range core.GetColumnsForTableAsColumns(i.meta, schema, tableName, i.dialect) {
			result.Fields = append(result.Fields, core.InspectField{
				Name: col.Name, Table: tableName, Schema: schema,
			})
		}
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
	return result
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

	relationRefs, _ := i.extractRelationRefsFromTableRefList(stmt.TableReferenceList())

	virtualTables := make(map[string]bool)
	result.Tables = i.convertRelationRefs(relationRefs, virtualTables)

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
			where, subs := i.extractWhereFieldsFromExpr(expr, relationRefs)
			result.Where = where
			result.Subqueries = append(result.Subqueries, subs...)
		}
	}
	return result
}

// ============================================
// DELETE
// ============================================

func (i *Inspector) inspectDelete(stmt mysql.IDeleteStatementContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpDelete}

	// sourceRefs covers every relation in scope for column resolution and the
	// WHERE clause; targetRefs are the tables actually being deleted from.
	var sourceRefs []core.RelationRef
	var targetRefs []core.RelationRef

	// Multi-table form (DELETE FROM list … or DELETE alias_list FROM list).
	if list := stmt.TableReferenceList(); list != nil {
		refs, _ := i.extractRelationRefsFromTableRefList(list)
		sourceRefs = refs
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

	result.Tables = i.convertRelationRefs(targetRefs, nil)

	if wc := stmt.WhereClause(); wc != nil {
		if expr := wc.Expr(); expr != nil {
			where, subs := i.extractWhereFieldsFromExpr(expr, sourceRefs)
			result.Where = where
			result.Subqueries = append(result.Subqueries, subs...)
		}
	}
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
	}
	return result
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
	parts := splitDotted(raw)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return defaultSchema, d.NormalizeIdentifier(parts[0])
	default:
		return d.NormalizeIdentifier(parts[0]), d.NormalizeIdentifier(parts[len(parts)-1])
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
	schema, table := splitQualifiedName(l.dialect, tr.GetText(), l.defaultSchema)
	if table == "" {
		return
	}
	ref := core.RelationRef{
		Schema:        schema,
		Table:         table,
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

	virtualTableFields := make(map[string][]core.InspectField)
	for idx, cte := range ctes {
		if idx < len(subqueries) {
			virtualTableFields[i.dialect.NormalizeIdentifier(cte.Table)] = subqueries[idx].Fields
		}
	}
	// Map FROM-subquery aliases to their results, in order.
	subqIdx := len(ctes)
	for alias := range subqueryColumns {
		if subqIdx < len(subqueries) {
			virtualTableFields[i.dialect.NormalizeIdentifier(alias)] = subqueries[subqIdx].Fields
			subqIdx++
		}
	}

	var resolved []core.InspectField
	for _, field := range fields {
		key := i.dialect.NormalizeIdentifier(field.Table)
		if underlying, ok := virtualTableFields[key]; ok {
			for _, uf := range underlying {
				if i.normalizeEquals(uf.Name, field.Name) {
					resolved = append(resolved, core.InspectField{
						Name:   field.Name,
						Alias:  field.Alias,
						Table:  uf.Table,
						Schema: uf.Schema,
					})
					break
				}
			}
			continue
		}
		resolved = append(resolved, field)
	}
	return resolved
}

func (i *Inspector) extractSelectFields(
	spec mysql.IQuerySpecificationContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	// MySQL's QuerySpecification has either "SELECT *" (with MULT_OPERATOR on
	// the SelectItemList) or a list of SelectItems.
	il := spec.SelectItemList()
	if il == nil {
		return nil
	}
	if il.MULT_OPERATOR() != nil && len(il.AllSelectItem()) == 0 {
		return i.expandStar(relationRefs, ctes, subqueryColumns, cteToSubqueryMap)
	}

	var fields []core.InspectField
	// Account for the leading "*, col1, col2" case: SELECT *, c1, c2.
	if il.MULT_OPERATOR() != nil {
		fields = append(fields, i.expandStar(relationRefs, ctes, subqueryColumns, cteToSubqueryMap)...)
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
	// table.*
	if tw := item.TableWild(); tw != nil {
		ids := tw.AllIdentifier()
		if len(ids) == 0 {
			return nil
		}
		prefix := i.dialect.NormalizeIdentifier(ids[len(ids)-1].GetText())
		return i.expandQualifiedStar(prefix, relationRefs, ctes, cteToSubqueryMap)
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

// expandStar expands SELECT * to all columns from all in-scope relations.
func (i *Inspector) expandStar(
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	var fields []core.InspectField
	for _, ref := range relationRefs {
		// CTE?
		matched := false
		for _, cte := range ctes {
			if i.normalizeEquals(cte.Table, ref.Table) {
				if cteResult, ok := cteToSubqueryMap[i.dialect.NormalizeIdentifier(cte.Table)]; ok {
					fields = append(fields, cteResult.Fields...)
				}
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// Subquery alias?
		if ref.Schema == "" && subqueryColumns != nil {
			if cols, ok := subqueryColumns[ref.Table]; ok {
				for _, col := range cols {
					fields = append(fields, core.InspectField{
						Name:   col.Name,
						Table:  ref.Table,
						Schema: i.meta.DefaultSchema,
					})
				}
				continue
			}
		}
		// Physical table.
		for _, col := range core.GetColumnsForTableAsColumns(i.meta, ref.Schema, ref.Table, i.dialect) {
			fields = append(fields, core.InspectField{
				Name:   col.Name,
				Table:  ref.Table,
				Schema: ref.Schema,
			})
		}
	}
	return fields
}

func (i *Inspector) expandQualifiedStar(
	tablePrefix string,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	target := i.dialect.NormalizeIdentifier(tablePrefix)
	for _, ref := range relationRefs {
		key := ref.Alias
		if key == "" {
			key = ref.Table
		}
		if i.dialect.NormalizeIdentifier(key) != target {
			continue
		}
		for _, cte := range ctes {
			if i.normalizeEquals(cte.Table, ref.Table) {
				if cteResult, ok := cteToSubqueryMap[i.dialect.NormalizeIdentifier(cte.Table)]; ok {
					return cteResult.Fields
				}
			}
		}
		var fields []core.InspectField
		for _, col := range core.GetColumnsForTableAsColumns(i.meta, ref.Schema, ref.Table, i.dialect) {
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
	if col.tablePrefix != "" {
		for _, ref := range relationRefs {
			key := ref.Alias
			if key == "" {
				key = ref.Table
			}
			if i.dialect.NormalizeIdentifier(key) != col.tablePrefix {
				continue
			}
			// CTE?
			for _, cte := range ctes {
				if i.normalizeEquals(cte.Table, ref.Table) {
					return i.resolveCTEColumnFromFields(col.name, cteToSubqueryMap[i.dialect.NormalizeIdentifier(cte.Table)])
				}
			}
			return &core.InspectField{
				Name:   col.name,
				Table:  ref.Table,
				Schema: ref.Schema,
			}
		}
	}

	// Unqualified: walk all relation refs.
	for _, ref := range relationRefs {
		// CTE?
		for _, cte := range ctes {
			if i.normalizeEquals(cte.Table, ref.Table) {
				for _, c := range cte.Columns {
					if i.normalizeEquals(c.Name, col.name) {
						return i.resolveCTEColumnFromFields(col.name, cteToSubqueryMap[i.dialect.NormalizeIdentifier(cte.Table)])
					}
				}
			}
		}
		// Subquery alias?
		if ref.Schema == "" && subqueryColumns != nil {
			if cols, ok := subqueryColumns[ref.Table]; ok {
				for _, c := range cols {
					if i.normalizeEquals(c.Name, col.name) {
						return &core.InspectField{
							Name:   col.name,
							Table:  ref.Table,
							Schema: i.meta.DefaultSchema,
						}
					}
				}
			}
		}
		// Physical table.
		for _, c := range core.GetColumnsForTableAsColumns(i.meta, ref.Schema, ref.Table, i.dialect) {
			if i.normalizeEquals(c.Name, col.name) {
				return &core.InspectField{
					Name:   col.name,
					Table:  ref.Table,
					Schema: ref.Schema,
				}
			}
		}
	}

	// Fallback: only one real table in scope.
	var known, unknown []core.RelationRef
	for _, ref := range relationRefs {
		if ref.Schema == "" {
			continue
		}
		if core.TableExistsInMetadata(i.meta, ref.Schema, ref.Table, i.dialect) {
			known = append(known, ref)
		} else {
			unknown = append(unknown, ref)
		}
	}
	if len(known) == 1 {
		return &core.InspectField{Name: col.name, Table: known[0].Table, Schema: known[0].Schema}
	}
	if len(known) == 0 && len(unknown) == 1 {
		return &core.InspectField{Name: col.name, Table: unknown[0].Table}
	}
	return nil
}

func (i *Inspector) resolveCTEColumnFromFields(columnName string, cteResult *core.InspectStatement) *core.InspectField {
	if cteResult != nil {
		for _, field := range cteResult.Fields {
			if i.normalizeEquals(field.Name, columnName) {
				return &core.InspectField{
					Name:   field.Name,
					Table:  field.Table,
					Schema: field.Schema,
				}
			}
		}
	}
	// Fallback: search metadata for any table with this column.
	for _, schema := range i.meta.Schemas {
		for _, table := range schema.Tables {
			for _, col := range table.Columns {
				if i.normalizeEquals(col.Name, columnName) {
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

// ============================================
// WHERE
// ============================================

func (i *Inspector) extractWhereFields(spec mysql.IQuerySpecificationContext, refs []core.RelationRef) ([]core.InspectField, []core.InspectStatement) {
	wc := spec.WhereClause()
	if wc == nil {
		return nil, nil
	}
	return i.extractWhereFieldsFromExpr(wc.Expr(), refs)
}

func (i *Inspector) extractWhereFieldsFromExpr(expr mysql.IExprContext, refs []core.RelationRef) ([]core.InspectField, []core.InspectStatement) {
	if expr == nil {
		return nil, nil
	}
	listener := &whereColumnListener{
		BaseMySQLParserListener: &mysql.BaseMySQLParserListener{},
		inspector:               i,
		relationRefs:            refs,
		seen:                    make(map[string]bool),
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, expr)

	subs := i.extractEmbeddedSubqueries(expr)
	return listener.fields, subs
}

type whereColumnListener struct {
	*mysql.BaseMySQLParserListener
	inspector     *Inspector
	relationRefs  []core.RelationRef
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
		for _, ref := range l.relationRefs {
			key := ref.Alias
			if key == "" {
				key = ref.Table
			}
			if l.inspector.dialect.NormalizeIdentifier(key) != cr.tablePrefix {
				continue
			}
			cols := core.GetColumnsForTableAsColumns(l.inspector.meta, ref.Schema, ref.Table, l.inspector.dialect)
			for _, c := range cols {
				if l.inspector.normalizeEquals(c.Name, cr.name) {
					resolved = &core.InspectField{Name: cr.name, Table: ref.Table, Schema: ref.Schema}
					break
				}
			}
			if resolved == nil {
				// Resolve to the prefix-matching ref even if metadata lacks the column.
				resolved = &core.InspectField{Name: cr.name, Table: ref.Table, Schema: ref.Schema}
			}
			break
		}
	} else {
		for _, ref := range l.relationRefs {
			cols := core.GetColumnsForTableAsColumns(l.inspector.meta, ref.Schema, ref.Table, l.inspector.dialect)
			for _, c := range cols {
				if l.inspector.normalizeEquals(c.Name, cr.name) {
					resolved = &core.InspectField{Name: cr.name, Table: ref.Table, Schema: ref.Schema}
					break
				}
			}
			if resolved != nil {
				break
			}
		}
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

// extractFromSubqueries collects InspectStatements for derived tables in FROM.
func (i *Inspector) extractFromSubqueries(spec mysql.IQuerySpecificationContext) []core.InspectStatement {
	fc := spec.FromClause()
	if fc == nil {
		return nil
	}
	listener := &fromSubqueryListener{
		BaseMySQLParserListener: &mysql.BaseMySQLParserListener{},
		inspector:               i,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, fc)
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
	for _, cte := range w.AllCommonTableExpression() {
		if cte == nil {
			continue
		}
		name := ""
		if id := cte.Identifier(); id != nil {
			name = i.dialect.NormalizeIdentifier(id.GetText())
		}
		var cols []core.Column
		if sq := cte.Subquery(); sq != nil {
			if sub := i.inspectSubquery(sq); sub != nil {
				subs = append(subs, *sub)
				for _, f := range sub.Fields {
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

func (i *Inspector) convertRelationRefs(refs []core.RelationRef, virtualTables map[string]bool) []core.InspectTable {
	var tables []core.InspectTable
	seen := make(map[string]bool)
	for _, ref := range refs {
		if ref.Schema == "" && ref.Table == "" {
			continue
		}
		if virtualTables != nil && virtualTables[i.dialect.NormalizeIdentifier(ref.Table)] {
			continue
		}
		schema := ref.Schema
		if schema != "" && !core.TableExistsInMetadata(i.meta, ref.Schema, ref.Table, i.dialect) {
			schema = ""
		}
		key := schema + "." + ref.Table
		if seen[key] {
			continue
		}
		seen[key] = true
		t := core.InspectTable{
			Name:      ref.Table,
			Schema:    schema,
			StartLine: ref.Line,
			StartCol:  ref.Col,
			EndCol:    ref.EndCol,
		}
		if ref.Alias != "" {
			t.Alias = &ref.Alias
		}
		tables = append(tables, t)
	}
	return tables
}
