package postgresql

import (
	"sort"
	"strings"

	core "github.com/selectDb/dialect/core"
	pg "github.com/selectDb/dialect/postgresql/parser"

	antlr "github.com/antlr4-go/antlr/v4"
)

// Inspector analyzes SQL statements and extracts structured information
type Inspector struct {
	dialect *Dialect
	meta    core.Metadata
}

// NewInspector creates a new PostgreSQL statement inspector
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

// normalizeEquals compares two identifiers after normalization
func (i *Inspector) normalizeEquals(a, b string) bool {
	return i.dialect.NormalizeIdentifier(a) == i.dialect.NormalizeIdentifier(b)
}

// Inspect analyzes SQL and returns structured results for each statement
func (i *Inspector) Inspect(sql string) []core.InspectStatement {
	lexer := i.dialect.CreateLexer(sql)
	tokenStream := antlr.NewCommonTokenStream(lexer, 0)
	parser := pg.NewPostgreSQLParser(tokenStream)
	parser.RemoveErrorListeners()

	root := parser.Root()
	if root == nil {
		return nil
	}

	stmtBlock := root.Stmtblock()
	if stmtBlock == nil {
		return nil
	}

	stmtMulti := stmtBlock.Stmtmulti()
	if stmtMulti == nil {
		return nil
	}

	statements := stmtMulti.AllStmt()
	if len(statements) == 0 {
		return nil
	}

	var results []core.InspectStatement
	for _, stmt := range statements {
		result := i.inspectStatement(stmt)
		if result != nil {
			results = append(results, *result)
		}
	}

	return results
}

// inspectStatement dispatches to the appropriate handler based on statement type
func (i *Inspector) inspectStatement(stmt pg.IStmtContext) *core.InspectStatement {
	if stmt == nil {
		return nil
	}

	// Handle SELECT statements
	if selectStmt := stmt.Selectstmt(); selectStmt != nil {
		return i.inspectSelect(selectStmt)
	}

	// Handle INSERT statements
	if insertStmt := stmt.Insertstmt(); insertStmt != nil {
		return i.inspectInsert(insertStmt)
	}

	// Handle UPDATE statements
	if updateStmt := stmt.Updatestmt(); updateStmt != nil {
		return i.inspectUpdate(updateStmt)
	}

	// Handle DELETE statements
	if deleteStmt := stmt.Deletestmt(); deleteStmt != nil {
		return i.inspectDelete(deleteStmt)
	}

	if truncateStmt := stmt.Truncatestmt(); truncateStmt != nil {
		return i.inspectTruncate(truncateStmt)
	}

	if dropStmt := stmt.Dropstmt(); dropStmt != nil {
		return i.inspectDrop(dropStmt)
	}

	if alterStmt := stmt.Altertablestmt(); alterStmt != nil {
		return i.inspectAlterTable(alterStmt)
	}

	if createStmt := stmt.Createstmt(); createStmt != nil {
		return i.inspectCreate(createStmt)
	}

	return nil
}

// inspectSelect analyzes a SELECT statement, including all UNION/INTERSECT/EXCEPT branches.
func (i *Inspector) inspectSelect(selectStmt pg.ISelectstmtContext) *core.InspectStatement {
	if selectStmt == nil {
		return nil
	}
	if selectNoParens := selectStmt.Select_no_parens(); selectNoParens != nil {
		return i.inspectSelectNoParens(selectNoParens)
	}
	if selectWithParens := selectStmt.Select_with_parens(); selectWithParens != nil {
		return i.inspectSelectWithParens(selectWithParens)
	}
	return nil
}

// inspectSelectNoParens analyzes a select_no_parens, processing all UNION/INTERSECT/EXCEPT branches.
func (i *Inspector) inspectSelectNoParens(selectNoParens pg.ISelect_no_parensContext) *core.InspectStatement {
	// CTEs are defined at the statement level and shared across all UNION branches.
	var ctes []core.RelationRef
	var cteSubqueries []core.InspectStatement
	if withClause := selectNoParens.With_clause(); withClause != nil {
		ctes, cteSubqueries = i.extractCTEsWithSubqueries(withClause)
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

	selectClause := selectNoParens.Select_clause()
	if selectClause == nil {
		return result
	}

	// Process every branch of UNION / INTERSECT / EXCEPT independently so that
	// column-to-table resolution uses only that branch's FROM clause.
	for _, intersect := range selectClause.AllSimple_select_intersect() {
		for _, primary := range intersect.AllSimple_select_pramary() {
			branch := i.inspectSelectPrimary(primary, ctes, cteSubqueries, cteToSubqueryMap)
			if branch == nil {
				continue
			}
			result.Tables = core.MergeInspectTables(result.Tables, branch.Tables)
			result.Fields = core.MergeInspectFields(result.Fields, branch.Fields)
			result.Where = core.MergeInspectFields(result.Where, branch.Where)
			result.Subqueries = append(result.Subqueries, branch.Subqueries...)
		}
	}

	return result
}

// inspectSelectPrimary analyzes a single simple_select_pramary, one branch of a UNION.
func (i *Inspector) inspectSelectPrimary(
	primary pg.ISimple_select_pramaryContext,
	ctes []core.RelationRef,
	cteSubqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) *core.InspectStatement {
	if primary == nil {
		return nil
	}

	relationRefs, subqueryColumns := i.extractRelationRefsFromPrimary(primary)

	fromSubqueries := i.extractFromSubqueriesFromPrimary(primary)

	// Virtual table names: CTEs and FROM subquery aliases must not appear in Tables.
	virtualTables := make(map[string]bool)
	for _, cte := range ctes {
		virtualTables[i.dialect.NormalizeIdentifier(cte.Table)] = true
	}
	for name := range subqueryColumns {
		virtualTables[i.dialect.NormalizeIdentifier(name)] = true
	}

	tables := i.convertRelationRefs(relationRefs, virtualTables)
	for _, subq := range cteSubqueries {
		tables = core.MergeInspectTables(tables, subq.Tables)
	}
	for _, subq := range fromSubqueries {
		tables = core.MergeInspectTables(tables, subq.Tables)
	}

	allSubqueries := append(cteSubqueries, fromSubqueries...)
	fields := i.extractSelectFieldsWithResolution(primary, relationRefs, ctes, subqueryColumns, allSubqueries, cteToSubqueryMap)

	where, whereSubqueries := i.extractWhereFieldsFromPrimary(primary, relationRefs)
	selectSubqueries := i.extractSelectListSubqueries(primary)

	subqueries := append(fromSubqueries, whereSubqueries...)
	subqueries = append(subqueries, selectSubqueries...)

	return &core.InspectStatement{
		Operation:  core.InspectOpSelect,
		Tables:     tables,
		Fields:     fields,
		Where:      where,
		Subqueries: subqueries,
	}
}

// inspectInsert analyzes an INSERT statement.
func (i *Inspector) inspectInsert(stmt pg.IInsertstmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpInsert}

	// Resolve target table (schema, name).
	schema, tableName := i.resolveQualifiedName(stmt.Insert_target().Qualified_name())
	if tableName == "" {
		return result
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	rest := stmt.Insert_rest()
	if rest == nil {
		return result
	}

	// Collect explicitly listed target columns.
	if colList := rest.Insert_column_list(); colList != nil {
		for _, item := range colList.AllInsert_column_item() {
			if colId := item.Colid(); colId != nil {
				name := i.dialect.NormalizeIdentifier(colId.GetText())
				result.Fields = append(result.Fields, core.InspectField{
					Name:   name,
					Table:  tableName,
					Schema: schema,
				})
			}
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

	// INSERT … SELECT: the grammar always wraps the source as a Selectstmt.
	// When it's a real SELECT (has tables), attach as subquery.
	// When it's VALUES, the Selectstmt has no FROM, so Tables is empty, skip.
	if selectStmt := rest.Selectstmt(); selectStmt != nil {
		if sub := i.inspectSelect(selectStmt); sub != nil && len(sub.Tables) > 0 {
			result.Subqueries = append(result.Subqueries, *sub)
		} else {
			// VALUES form: walk the expression tree for embedded subqueries.
			result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(rest)...)
		}
	}

	// RETURNING clause columns require SELECT permission.
	if ret := stmt.Returning_clause(); ret != nil {
		if targetList := ret.Target_list(); targetList != nil {
			for _, el := range targetList.AllTarget_el() {
				field := i.extractFieldFromTarget(el, []core.RelationRef{{
					Table:  tableName,
					Schema: schema,
				}}, nil, nil, nil)
				if field != nil && !containsField(result.Fields, *field) {
					result.Fields = append(result.Fields, *field)
				}
			}
		}
	}

	return result
}

// inspectTruncate analyzes a TRUNCATE statement.
func (i *Inspector) inspectTruncate(stmt pg.ITruncatestmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpTruncate}
	if list := stmt.Relation_expr_list(); list != nil {
		for _, rel := range list.AllRelation_expr() {
			schema, table := i.resolveQualifiedName(rel.Qualified_name())
			if table != "" {
				result.Tables = append(result.Tables, core.InspectTable{Name: table, Schema: schema})
			}
		}
	}
	return result
}

// inspectDrop analyzes a DROP TABLE statement.
// Only TABLE drops are handled; other DROP variants (INDEX, VIEW, etc.) return op only.
func (i *Inspector) inspectDrop(stmt pg.IDropstmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpDrop}
	if nameList := stmt.Any_name_list(); nameList != nil {
		for _, anyName := range nameList.AllAny_name() {
			schema, table := i.resolveAnyName(anyName)
			if table != "" {
				result.Tables = append(result.Tables, core.InspectTable{Name: table, Schema: schema})
			}
		}
	}
	return result
}

// inspectAlterTable analyzes an ALTER TABLE statement.
func (i *Inspector) inspectAlterTable(stmt pg.IAltertablestmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpAlter}
	if rel := stmt.Relation_expr(); rel != nil {
		schema, table := i.resolveQualifiedName(rel.Qualified_name())
		if table != "" {
			result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
		}
	}
	return result
}

// inspectCreate analyzes a CREATE TABLE statement.
func (i *Inspector) inspectCreate(stmt pg.ICreatestmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpCreate}
	// AllQualified_name returns [new_table] or [new_table, of_type], first is the table being created.
	if names := stmt.AllQualified_name(); len(names) > 0 {
		schema, table := i.resolveQualifiedName(names[0])
		if table != "" {
			result.Tables = []core.InspectTable{{Name: table, Schema: schema}}
		}
	}
	return result
}

// resolveAnyName extracts (schema, table) from an any_name node (used in DROP statements).
// any_name = colid attrs? where attrs = ('.' attr_name)*
func (i *Inspector) resolveAnyName(anyName pg.IAny_nameContext) (schema, table string) {
	if anyName == nil {
		return "", ""
	}
	defaultSchema := core.GetDefaultSchema(i.meta)
	colId := anyName.Colid()
	if colId == nil {
		return "", ""
	}
	first := i.dialect.NormalizeIdentifier(colId.GetText())
	if attrs := anyName.Attrs(); attrs != nil {
		attrNames := attrs.AllAttr_name()
		if len(attrNames) > 0 {
			return first, i.dialect.NormalizeIdentifier(attrNames[0].GetText())
		}
	}
	return defaultSchema, first
}

// resolveQualifiedName extracts (schema, table) from a qualified_name node.
// Schema defaults to DefaultSchema when not specified.
func (i *Inspector) resolveQualifiedName(q pg.IQualified_nameContext) (schema, table string) {
	if q == nil {
		return "", ""
	}
	defaultSchema := core.GetDefaultSchema(i.meta)
	colId := q.Colid()
	if colId == nil {
		return "", ""
	}
	if indirection := q.Indirection(); indirection != nil {
		schema = i.dialect.NormalizeIdentifier(colId.GetText())
		els := indirection.AllIndirection_el()
		if len(els) > 0 {
			if attr := els[0].Attr_name(); attr != nil {
				table = i.dialect.NormalizeIdentifier(attr.GetText())
			}
		}
	} else {
		schema = defaultSchema
		table = i.dialect.NormalizeIdentifier(colId.GetText())
	}
	return schema, table
}

// containsField reports whether fields already contains f by (name, table, schema).
func containsField(fields []core.InspectField, f core.InspectField) bool {
	for _, existing := range fields {
		if existing.Name == f.Name && existing.Table == f.Table && existing.Schema == f.Schema {
			return true
		}
	}
	return false
}

// inspectUpdate analyzes an UPDATE statement.
func (i *Inspector) inspectUpdate(stmt pg.IUpdatestmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpUpdate}

	// Resolve target table from relation_expr_opt_alias.
	relOptAlias := stmt.Relation_expr_opt_alias()
	if relOptAlias == nil {
		return result
	}
	schema, tableName := i.resolveQualifiedName(relOptAlias.Relation_expr().Qualified_name())
	if tableName == "" {
		return result
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	// The target table ref used for column resolution.
	targetRef := core.RelationRef{Table: tableName, Schema: schema}

	// Collect SET columns from set_clause_list.
	if setList := stmt.Set_clause_list(); setList != nil {
		for _, clause := range setList.AllSet_clause() {
			if target := clause.Set_target(); target != nil {
				if colId := target.Colid(); colId != nil {
					name := i.dialect.NormalizeIdentifier(colId.GetText())
					result.Fields = append(result.Fields, core.InspectField{
						Name:   name,
						Table:  tableName,
						Schema: schema,
					})
				}
			}
			// Walk the RHS expression for embedded subqueries.
			if expr := clause.A_expr(); expr != nil {
				result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(expr)...)
			}
		}
	}

	// WHERE clause fields.
	if whereClause := stmt.Where_or_current_clause(); whereClause != nil {
		if expr := whereClause.A_expr(); expr != nil {
			listener := &whereColumnExtractorListener{
				BasePostgreSQLParserListener: &pg.BasePostgreSQLParserListener{},
				inspector:                    i,
				relationRefs:                 []core.RelationRef{targetRef},
				fields:                       []core.InspectField{},
				seenFields:                   make(map[string]bool),
			}
			antlr.ParseTreeWalkerDefault.Walk(listener, expr)
			result.Where = listener.fields

			// Subqueries embedded in WHERE.
			result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(expr)...)
		}
	}

	return result
}

// inspectDelete analyzes a DELETE statement.
func (i *Inspector) inspectDelete(stmt pg.IDeletestmtContext) *core.InspectStatement {
	result := &core.InspectStatement{Operation: core.InspectOpDelete}

	relOptAlias := stmt.Relation_expr_opt_alias()
	if relOptAlias == nil {
		return result
	}
	schema, tableName := i.resolveQualifiedName(relOptAlias.Relation_expr().Qualified_name())
	if tableName == "" {
		return result
	}
	result.Tables = []core.InspectTable{{Name: tableName, Schema: schema}}

	if whereClause := stmt.Where_or_current_clause(); whereClause != nil {
		if expr := whereClause.A_expr(); expr != nil {
			listener := &whereColumnExtractorListener{
				BasePostgreSQLParserListener: &pg.BasePostgreSQLParserListener{},
				inspector:                    i,
				relationRefs:                 []core.RelationRef{{Table: tableName, Schema: schema}},
				fields:                       []core.InspectField{},
				seenFields:                   make(map[string]bool),
			}
			antlr.ParseTreeWalkerDefault.Walk(listener, expr)
			result.Where = listener.fields
			result.Subqueries = append(result.Subqueries, i.extractEmbeddedSubqueries(expr)...)
		}
	}

	return result
}

// extractRelationRefsFromPrimary extracts table references from a simple_select_pramary.
func (i *Inspector) extractRelationRefsFromPrimary(primary pg.ISimple_select_pramaryContext) ([]core.RelationRef, map[string][]core.Column) {
	if primary == nil {
		return nil, nil
	}
	fromClause := primary.From_clause()
	if fromClause == nil {
		return nil, nil
	}
	fw := &fromWalker{dialect: i.dialect, meta: i.meta}
	return fw.walk(fromClause)
}

// extractSelectFieldsWithResolution extracts fields and resolves virtual tables to underlying tables
func (i *Inspector) extractSelectFieldsWithResolution(
	primary pg.ISimple_select_pramaryContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	subqueries []core.InspectStatement,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	fields := i.extractSelectFields(primary, relationRefs, ctes, subqueryColumns, cteToSubqueryMap)

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

// extractSelectFields extracts fields from a simple_select_pramary.
func (i *Inspector) extractSelectFields(
	primary pg.ISimple_select_pramaryContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	targetList := primary.Target_list()
	if targetList == nil {
		if optTargetList := primary.Opt_target_list(); optTargetList != nil {
			targetList = optTargetList.Target_list()
		}
		if targetList == nil {
			return nil
		}
	}

	var fields []core.InspectField
	targetElements := targetList.AllTarget_el()

	for _, targetEl := range targetElements {
		// Handle SELECT *
		if starCtx, ok := targetEl.(*pg.Target_starContext); ok && starCtx.STAR() != nil {
			fields = append(fields, i.expandStar(relationRefs, ctes, subqueryColumns, cteToSubqueryMap)...)
			continue
		}

		// Handle regular column references (may include table.* which we detect below)
		for _, field := range i.extractFieldsFromTarget(targetEl, relationRefs, ctes, subqueryColumns, cteToSubqueryMap) {
			// Check if this is a qualified star (table.*)
			if field.Name == "*" && field.Table != "" {
				fields = append(fields, i.expandQualifiedStar(field.Table, relationRefs, ctes, cteToSubqueryMap)...)
				continue
			}
			fields = append(fields, field)
		}
	}

	return fields
}

// expandStar expands SELECT * to all columns from all tables
func (i *Inspector) expandStar(
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	var fields []core.InspectField

	for _, ref := range relationRefs {
		// Check if this is a CTE reference
		for _, cte := range ctes {
			if i.dialect.NormalizeIdentifier(cte.Table) == i.dialect.NormalizeIdentifier(ref.Table) {
				// Use the CTE's subquery result to resolve columns properly
				cteKey := i.dialect.NormalizeIdentifier(cte.Table)
				if cteResult, ok := cteSubqueryMap[cteKey]; ok {
					// Use fields from the CTE's InspectStatement
					fields = append(fields, cteResult.Fields...)
				} else {
					// Fallback: resolve each column individually
					for _, col := range cte.Columns {
						// Try to find the column in the CTE's fields
						// This is a fallback - ideally cteSubqueryMap should always have the result
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

// extractFieldsFromTarget extracts all fields from a target element.
// A single target element may reference multiple columns (e.g. c1 + c2, COALESCE(c1, c2)).
// The alias (if any) is attached only to the first field, it describes the output expression,
// not individual source columns.
func (i *Inspector) extractFieldsFromTarget(
	targetEl pg.ITarget_elContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) []core.InspectField {
	if targetEl == nil {
		return nil
	}

	switch t := targetEl.(type) {
	case *pg.Target_columnrefContext:
		if colRef := t.Columnref(); colRef != nil {
			name, prefix := i.extractColumnInfo(colRef)
			if f := i.resolveColumn(name, prefix, nil, relationRefs, ctes, subqueryColumns, cteToSubqueryMap); f != nil {
				if tok := colRef.GetStop(); tok != nil {
					f.StartLine = tok.GetLine()
					f.StartCol = tok.GetColumn()
					f.EndCol = tok.GetColumn() + len(tok.GetText())
				}
				return []core.InspectField{*f}
			}
		}

	case *pg.Target_labelContext:
		var alias *string
		if aliasCtx := t.Target_alias(); aliasCtx != nil {
			var aliasText string
			if collabel := aliasCtx.Collabel(); collabel != nil {
				aliasText = collabel.GetText()
			} else if identifier := aliasCtx.Identifier(); identifier != nil {
				aliasText = identifier.GetText()
			}
			if aliasText != "" {
				aliasText = i.dialect.NormalizeIdentifier(aliasText)
				alias = &aliasText
			}
		}
		if expr := t.A_expr(); expr != nil {
			cols := i.extractAllColumnsFromExpr(expr)
			var fields []core.InspectField
			for _, col := range cols {
				// Alias describes the output expression, not individual source columns.
				// Attach it only when the expression resolves to a single source column.
				var a *string
				if len(cols) == 1 {
					a = alias
				}
				if f := i.resolveColumn(col.name, col.tablePrefix, a, relationRefs, ctes, subqueryColumns, cteToSubqueryMap); f != nil {
					f.StartLine = col.line
					f.StartCol = col.col
					f.EndCol = col.endCol
					fields = append(fields, *f)
				}
			}
			return fields
		}
	}

	return nil
}

// extractFieldFromTarget is kept for callers that only need one field (RETURNING, etc.).
func (i *Inspector) extractFieldFromTarget(
	targetEl pg.ITarget_elContext,
	relationRefs []core.RelationRef,
	ctes []core.RelationRef,
	subqueryColumns map[string][]core.Column,
	cteToSubqueryMap map[string]*core.InspectStatement,
) *core.InspectField {
	fields := i.extractFieldsFromTarget(targetEl, relationRefs, ctes, subqueryColumns, cteToSubqueryMap)
	if len(fields) == 0 {
		return nil
	}
	return &fields[0]
}

// columnRef holds a resolved column name and optional table prefix from an expression.
type columnRef struct {
	name        string
	tablePrefix string
	line        int // 1-based, 0 = unknown
	col         int // 0-based
	endCol      int // 0-based exclusive
}

// extractAllColumnsFromExpr walks an expression and returns all column references in order.
func (i *Inspector) extractAllColumnsFromExpr(expr pg.IA_exprContext) []columnRef {
	if expr == nil {
		return nil
	}
	listener := &allColumnsExtractorListener{
		BasePostgreSQLParserListener: &pg.BasePostgreSQLParserListener{},
		inspector:                    i,
		seen:                         make(map[string]bool),
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, expr)
	return listener.cols
}

// allColumnsExtractorListener collects all column references from an expression, in order, deduplicated.
// It does not descend into embedded subqueries, those are handled separately.
type allColumnsExtractorListener struct {
	*pg.BasePostgreSQLParserListener
	inspector     *Inspector
	cols          []columnRef
	seen          map[string]bool
	subqueryDepth int
}

func (l *allColumnsExtractorListener) EnterSelect_with_parens(_ *pg.Select_with_parensContext) {
	l.subqueryDepth++
}

func (l *allColumnsExtractorListener) ExitSelect_with_parens(_ *pg.Select_with_parensContext) {
	l.subqueryDepth--
}

func (l *allColumnsExtractorListener) EnterColumnref(ctx *pg.ColumnrefContext) {
	if ctx == nil || l.subqueryDepth > 0 {
		return
	}
	name, prefix := l.inspector.extractColumnInfo(ctx)
	if name == "" {
		return
	}
	key := prefix + "." + name
	if !l.seen[key] {
		l.seen[key] = true
		ref := columnRef{name: name, tablePrefix: prefix}
		if tok := ctx.GetStop(); tok != nil {
			ref.line = tok.GetLine()
			ref.col = tok.GetColumn()
			ref.endCol = tok.GetColumn() + len(tok.GetText())
		}
		l.cols = append(l.cols, ref)
	}
}

// extractEmbeddedSubqueries walks an arbitrary AST node and returns InspectStatements
// for every select_with_parens found inside expression trees (e.g. VALUES with subqueries).
func (i *Inspector) extractEmbeddedSubqueries(ctx antlr.ParseTree) []core.InspectStatement {
	listener := &embeddedSubqueryListener{
		BasePostgreSQLParserListener: &pg.BasePostgreSQLParserListener{},
		inspector:                    i,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, ctx)
	return listener.results
}

type embeddedSubqueryListener struct {
	*pg.BasePostgreSQLParserListener
	inspector     *Inspector
	results       []core.InspectStatement
	subqueryDepth int
}

func (l *embeddedSubqueryListener) EnterSelect_with_parens(ctx *pg.Select_with_parensContext) {
	// Only collect direct subqueries (depth 0). Nested ones are captured recursively
	// inside each collected subquery's own inspection.
	if l.subqueryDepth == 0 {
		if sub := l.inspector.inspectSelectWithParens(ctx); sub != nil && len(sub.Tables) > 0 {
			l.results = append(l.results, *sub)
		}
	}
	l.subqueryDepth++
}

func (l *embeddedSubqueryListener) ExitSelect_with_parens(_ *pg.Select_with_parensContext) {
	l.subqueryDepth--
}

// extractColumnInfo extracts column name and optional table prefix from a column reference
func (i *Inspector) extractColumnInfo(colRef pg.IColumnrefContext) (string, string) {
	if colRef == nil {
		return "", ""
	}

	fullText := colRef.GetText()
	if parts := core.SplitQualified(fullText, '"'); len(parts) > 1 {
		// Qualified: table.column or schema.table.column
		return i.dialect.NormalizeIdentifier(parts[len(parts)-1]), i.dialect.NormalizeIdentifier(parts[len(parts)-2])
	}

	// Unqualified column
	if colId := colRef.Colid(); colId != nil {
		return i.dialect.NormalizeIdentifier(colId.GetText()), ""
	}

	return i.dialect.NormalizeIdentifier(fullText), ""
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

				// Look up canonical column name from schema metadata to preserve original case.
				canonicalName := normalizedCol
				tableCols := core.GetColumnsForTableAsColumns(i.meta, ref.Schema, ref.Table, i.dialect)
				for _, col := range tableCols {
					if i.dialect.NormalizeIdentifier(col.Name) == normalizedCol {
						canonicalName = col.Name
						break
					}
				}

				return &core.InspectField{
					Name:   canonicalName,
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
							Name:   col.Name,
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
					Name:   col.Name,
					Alias:  alias,
					Table:  ref.Table,
					Schema: ref.Schema,
				}
			}
		}
	}

	// Column not found in metadata. When a table prefix was given, we know which table
	// it targets, return it unresolved so the permission checker can see it.
	// For unqualified columns with a single real table ref, attribute to that table.
	if tablePrefix != "" {
		for _, ref := range relationRefs {
			refKey := ref.Alias
			if refKey == "" {
				refKey = ref.Table
			}
			if i.dialect.NormalizeIdentifier(refKey) == i.dialect.NormalizeIdentifier(tablePrefix) {
				return &core.InspectField{
					Name:   columnName,
					Alias:  alias,
					Table:  ref.Table,
					Schema: ref.Schema,
				}
			}
		}
	} else {
		// Count real table refs (skip CTEs and subquery refs which have Schema=="")
		var realRefs []core.RelationRef
		for _, ref := range relationRefs {
			if ref.Schema != "" {
				realRefs = append(realRefs, ref)
			}
		}
		if len(realRefs) == 1 {
			// Use "" schema when the table is not in metadata, matching convertRelationRefs behaviour.
			schema := realRefs[0].Schema
			if !core.TableExistsInMetadata(i.meta, schema, realRefs[0].Table, i.dialect) {
				schema = ""
			}
			return &core.InspectField{
				Name:   columnName,
				Alias:  alias,
				Table:  realRefs[0].Table,
				Schema: schema,
			}
		}
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

// extractSelectListSubqueries returns InspectStatements for any scalar subqueries embedded
// in the SELECT projection (e.g. SELECT c1, (SELECT c3 FROM t2) FROM t1).
func (i *Inspector) extractSelectListSubqueries(primary pg.ISimple_select_pramaryContext) []core.InspectStatement {
	if primary == nil {
		return nil
	}
	targetList := primary.Target_list()
	if targetList == nil {
		if optTargetList := primary.Opt_target_list(); optTargetList != nil {
			targetList = optTargetList.Target_list()
		}
		if targetList == nil {
			return nil
		}
	}
	var subqueries []core.InspectStatement
	for _, targetEl := range targetList.AllTarget_el() {
		subqueries = append(subqueries, i.extractEmbeddedSubqueries(targetEl)...)
	}
	return subqueries
}

// extractWhereFieldsFromPrimary extracts column references and embedded subqueries
// from the WHERE clause of a simple_select_pramary.
func (i *Inspector) extractWhereFieldsFromPrimary(primary pg.ISimple_select_pramaryContext, relationRefs []core.RelationRef) ([]core.InspectField, []core.InspectStatement) {
	if primary == nil {
		return nil, nil
	}

	whereClause := primary.Where_clause()
	if whereClause == nil {
		return nil, nil
	}

	whereExpr := whereClause.A_expr()
	if whereExpr == nil {
		return nil, nil
	}

	listener := &whereColumnExtractorListener{
		BasePostgreSQLParserListener: &pg.BasePostgreSQLParserListener{},
		inspector:                    i,
		relationRefs:                 relationRefs,
		fields:                       []core.InspectField{},
		seenFields:                   make(map[string]bool),
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, whereExpr)

	subqueries := i.extractEmbeddedSubqueries(whereExpr)

	return listener.fields, subqueries
}

// whereColumnExtractorListener extracts column references from WHERE expressions.
// It does not descend into subqueries, those are handled separately as InspectStatements.
type whereColumnExtractorListener struct {
	*pg.BasePostgreSQLParserListener
	inspector     *Inspector
	relationRefs  []core.RelationRef
	fields        []core.InspectField
	seenFields    map[string]bool
	subqueryDepth int
}

func (l *whereColumnExtractorListener) EnterSelect_with_parens(_ *pg.Select_with_parensContext) {
	l.subqueryDepth++
}

func (l *whereColumnExtractorListener) ExitSelect_with_parens(_ *pg.Select_with_parensContext) {
	l.subqueryDepth--
}

func (l *whereColumnExtractorListener) EnterColumnref(ctx *pg.ColumnrefContext) {
	if ctx == nil || l.subqueryDepth > 0 {
		return
	}

	// Extract column name and table prefix
	columnName, tablePrefix := l.inspector.extractColumnInfo(ctx)

	if columnName == "" {
		return
	}

	normalizedCol := l.inspector.dialect.NormalizeIdentifier(columnName)

	// Resolve the column to its table
	var resolvedField *core.InspectField
	if tablePrefix != "" {
		// Qualified column reference - resolve using table prefix
		for _, ref := range l.relationRefs {
			refKey := ref.Alias
			if refKey == "" {
				refKey = ref.Table
			}
			if l.inspector.dialect.NormalizeIdentifier(refKey) == l.inspector.dialect.NormalizeIdentifier(tablePrefix) {
				// Check if column exists in this table
				tableCols := core.GetColumnsForTableAsColumns(l.inspector.meta, ref.Schema, ref.Table, l.inspector.dialect)
				for _, col := range tableCols {
					if l.inspector.dialect.NormalizeIdentifier(col.Name) == normalizedCol {
						resolvedField = &core.InspectField{
							Name:   col.Name,
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
		// Unqualified column reference - search all tables
		for _, ref := range l.relationRefs {
			tableCols := core.GetColumnsForTableAsColumns(l.inspector.meta, ref.Schema, ref.Table, l.inspector.dialect)
			for _, col := range tableCols {
				if l.inspector.dialect.NormalizeIdentifier(col.Name) == normalizedCol {
					resolvedField = &core.InspectField{
						Name:   col.Name,
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

	// Add field if resolved and not already seen
	if resolvedField != nil {
		key := resolvedField.Schema + "." + resolvedField.Table + "." + resolvedField.Name
		if !l.seenFields[key] {
			l.seenFields[key] = true
			l.fields = append(l.fields, *resolvedField)
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

		// If the table is not found in metadata the schema is unknown, use "" so the
		// permission checker can treat it as unresolved and deny the query.
		schema := ref.Schema
		if !core.TableExistsInMetadata(i.meta, schema, ref.Table, i.dialect) {
			schema = ""
		}

		// Deduplicate
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
func (i *Inspector) extractCTEsWithSubqueries(withClause pg.IWith_clauseContext) ([]core.RelationRef, []core.InspectStatement) {
	if withClause == nil {
		return nil, nil
	}

	var ctes []core.RelationRef
	var subqueries []core.InspectStatement

	cteList := withClause.Cte_list()
	if cteList == nil {
		return ctes, subqueries
	}

	cteElements := cteList.AllCommon_table_expr()
	for _, cteEl := range cteElements {
		if cteEl == nil {
			continue
		}

		// Get CTE name
		cteName := ""
		if nameCtx := cteEl.Name(); nameCtx != nil {
			cteName = i.dialect.NormalizeIdentifier(nameCtx.GetText())
		}

		// Inspect the CTE body to get its InspectStatement and columns
		var cteColumns []core.Column
		if preparableStmt := cteEl.Preparablestmt(); preparableStmt != nil {
			if selectStmt := preparableStmt.Selectstmt(); selectStmt != nil {
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

// extractFromSubqueriesFromPrimary extracts InspectStatements from subqueries in the FROM clause of a simple_select_pramary.
func (i *Inspector) extractFromSubqueriesFromPrimary(primary pg.ISimple_select_pramaryContext) []core.InspectStatement {
	if primary == nil {
		return nil
	}
	fromClause := primary.From_clause()
	if fromClause == nil {
		return nil
	}
	return i.extractSubqueriesFromFromClause(fromClause)
}

// extractSubqueriesFromFromClause recursively finds subqueries in FROM clause
func (i *Inspector) extractSubqueriesFromFromClause(fromClause pg.IFrom_clauseContext) []core.InspectStatement {
	var subqueries []core.InspectStatement

	fromList := fromClause.From_list()
	if fromList == nil {
		return subqueries
	}

	relationRefs := fromList.AllTable_ref()
	for _, relationRef := range relationRefs {
		subqueries = append(subqueries, i.extractSubqueriesFromRelationRef(relationRef)...)
	}

	return subqueries
}

// extractSubqueriesFromRelationRef extracts subqueries from a relation reference
func (i *Inspector) extractSubqueriesFromRelationRef(relationRef pg.ITable_refContext) []core.InspectStatement {
	var subqueries []core.InspectStatement

	if relationRef == nil {
		return subqueries
	}

	// Check for subquery (select_with_parens) directly on relation reference
	if selectWithParens := relationRef.Select_with_parens(); selectWithParens != nil {
		if subResult := i.inspectSelectWithParens(selectWithParens); subResult != nil {
			subqueries = append(subqueries, *subResult)
		}
	}

	// Check nested relation reference (parenthesized)
	if nestedTableRef := relationRef.Table_ref(); nestedTableRef != nil {
		subqueries = append(subqueries, i.extractSubqueriesFromRelationRef(nestedTableRef)...)
	}

	// Check joined tables recursively
	for _, joinedTable := range relationRef.AllJoined_table() {
		if joinedTable != nil {
			if joinedRef := joinedTable.Table_ref(); joinedRef != nil {
				subqueries = append(subqueries, i.extractSubqueriesFromRelationRef(joinedRef)...)
			}
		}
	}

	return subqueries
}

// inspectSelectWithParens inspects a parenthesized SELECT
func (i *Inspector) inspectSelectWithParens(ctx pg.ISelect_with_parensContext) *core.InspectStatement {
	if ctx == nil {
		return nil
	}

	if selectNoParens := ctx.Select_no_parens(); selectNoParens != nil {
		return i.inspectSelectNoParens(selectNoParens)
	}

	if nested := ctx.Select_with_parens(); nested != nil {
		return i.inspectSelectWithParens(nested)
	}

	return nil
}

// fromWalker extracts table references and subquery columns from a FROM clause.
// Used by the Inspector to resolve table permissions.
type fromWalker struct {
	dialect *Dialect
	meta    core.Metadata
}

func (fw *fromWalker) walk(fromClause pg.IFrom_clauseContext) ([]core.RelationRef, map[string][]core.Column) {
	if fromClause == nil {
		return nil, nil
	}

	fromList := fromClause.From_list()
	if fromList == nil {
		return nil, nil
	}

	var refs []core.RelationRef
	subqueryColumns := make(map[string][]core.Column)
	// Determine effective schema: use CurrentSchema if set, otherwise DefaultSchema
	effectiveSchema := fw.meta.CurrentSchema
	if effectiveSchema == "" {
		effectiveSchema = fw.meta.DefaultSchema
	}

	relationRefs := fromList.AllTable_ref()
	for _, relationRef := range relationRefs {
		// Process the main table reference
		if subquery := relationRef.Select_with_parens(); subquery != nil {
			// Handle subquery - create a virtual table reference
			ref := fw.processSubqueryRelationRef(relationRef, effectiveSchema)
			if ref != nil {
				refs = append(refs, *ref)
			}

			// Parse the subquery to extract its inner table references and columns
			subqueryRefs, columns := fw.parseSubqueryFromAST(subquery)
			refs = append(refs, subqueryRefs...)

			// Store columns for this subquery (use Table as key since alias is in Table field)
			if ref != nil && len(columns) > 0 && ref.Table != "" {
				subqueryColumns[ref.Table] = columns
			}
		} else {
			// Handle regular table reference
			ref := fw.processRelationRef(relationRef, effectiveSchema)
			if ref != nil {
				refs = append(refs, *ref)
			}
		}

		// Process joined tables
		joinedTables := relationRef.AllJoined_table()
		for _, joinedTable := range joinedTables {
			if joinedTableRef := joinedTable.Table_ref(); joinedTableRef != nil {
				if subquery := joinedTableRef.Select_with_parens(); subquery != nil {
					// Handle subquery in JOIN
					ref := fw.processSubqueryRelationRef(joinedTableRef, effectiveSchema)
					if ref != nil {
						refs = append(refs, *ref)
					}

					// Parse the subquery to extract its inner table references and columns
					subqueryRefs, columns := fw.parseSubqueryFromAST(subquery)
					refs = append(refs, subqueryRefs...)

					// Store columns for this subquery
					// For subqueries, use Table as the key (since alias is now in Table field)
					if ref != nil && len(columns) > 0 {
						if ref.Table != "" {
							subqueryColumns[ref.Table] = columns
						}
					}
				} else {
					// Handle regular table reference in JOIN
					ref := fw.processRelationRef(joinedTableRef, effectiveSchema)
					if ref != nil {
						refs = append(refs, *ref)
					}
				}
			}
		}
	}

	return refs, subqueryColumns
}

func (fw *fromWalker) parseSubqueryFromAST(subquery pg.ISelect_with_parensContext) ([]core.RelationRef, []core.Column) {
	var refs []core.RelationRef
	var columns []core.Column

	// Get the select statement from the subquery
	if selectStmt := subquery.Select_no_parens(); selectStmt != nil {
		// Get the FROM clause from the subquery
		selectClause := selectStmt.Select_clause()
		if selectClause != nil {
			simpleSelects := selectClause.AllSimple_select_intersect()
			if len(simpleSelects) > 0 {
				simpleSelectPrimaries := simpleSelects[0].AllSimple_select_pramary()
				if len(simpleSelectPrimaries) > 0 {
					simpleSelectPrimary := simpleSelectPrimaries[0]

					// Get FROM clause from the subquery
					var nestedSubqueryColumns map[string][]core.Column
					if fromClause := simpleSelectPrimary.From_clause(); fromClause != nil {
						// Recursively parse the FROM clause
						subqueryRefs, nestedSubqueryCols := fw.walk(fromClause)
						refs = append(refs, subqueryRefs...)
						nestedSubqueryColumns = nestedSubqueryCols
					}

					// Get SELECT clause from the subquery to extract columns
					targetList := simpleSelectPrimary.Target_list()
					if targetList == nil {
						if optTargetList := simpleSelectPrimary.Opt_target_list(); optTargetList != nil {
							targetList = optTargetList.Target_list()
						}
					}

					if targetList != nil {
						// Extract columns from the target list
						targetElements := targetList.AllTarget_el()

						// Get table references from the subquery for SELECT * expansion
						var subqueryRelationRefs []core.RelationRef
						if fromClause := simpleSelectPrimary.From_clause(); fromClause != nil {
							subqueryRelationRefs, nestedSubqueryColumns = fw.walk(fromClause)
						}

						for _, targetEl := range targetElements {
							// Handle SELECT * specially
							if starCtx, ok := targetEl.(*pg.Target_starContext); ok && starCtx.STAR() != nil {
								// Expand SELECT * to all columns from all tables
								for _, ref := range subqueryRelationRefs {
									// Check if this is a virtual table (subquery alias)
									if ref.Schema == "" && nestedSubqueryColumns != nil {
										if vtabCols, ok := nestedSubqueryColumns[ref.Table]; ok {
											// Use columns from virtual table
											columns = append(columns, vtabCols...)
											continue
										}
									}
									// Regular table - lookup in metadata
									columns = append(columns, core.GetColumnsForTableAsColumns(fw.meta, ref.Schema, ref.Table, fw.dialect)...)
								}
							} else {
								column := fw.processTargetElementToColumn(targetEl, subqueryRelationRefs)
								if column != nil {
									columns = append(columns, *column)
								}
							}
						}
					}
				}
			}
		}
	}

	return refs, columns
}

func (fw *fromWalker) processTargetElementToColumn(
	targetEl pg.ITarget_elContext,
	relationRefs []core.RelationRef,
) *core.Column {
	if targetEl == nil {
		return nil
	}

	// Handle different types of target elements
	switch t := targetEl.(type) {
	case *pg.Target_starContext:
		// SELECT * - return nil as this is handled elsewhere
		return nil
	case *pg.Target_columnrefContext:
		if colRef := t.Columnref(); colRef != nil {
			cols := fw.dialect.processColumnRef(colRef, relationRefs, fw.meta)
			if len(cols) == 0 {
				return nil
			}
			c := cols[0]
			return &c
		}
	case *pg.Target_labelContext:
		// Target with label (alias)
		if expr := t.A_expr(); expr != nil {
			// Get the alias
			alias := fw.extractAliasFromTargetLabel(t)

			// Try to extract column name from expression
			if columnRef := fw.extractColumnRefFromExpression(expr); columnRef != nil {
				columnName := fw.extractColumnNameFromColumnRef(columnRef)
				if columnName != "" {
					// Use alias if present, otherwise use column name
					finalName := alias
					if finalName == "" {
						finalName = columnName
					}

					// Look up column type from metadata
					columnType := "unknown"
					nullable := true

					// Try to find the column in metadata
					for _, schema := range fw.meta.Schemas {
						for _, table := range schema.Tables {
							for _, col := range table.Columns {
								if fw.dialect.NormalizeIdentifier(col.Name) == fw.dialect.NormalizeIdentifier(columnName) {
									columnType = col.Type
									nullable = col.Nullable
									break
								}
							}
						}
					}

					return &core.Column{
						Name:     finalName,
						Type:     columnType,
						Nullable: nullable,
					}
				}
			}

			// Fallback: create a column with the alias
			if alias != "" {
				return &core.Column{
					Name:     alias,
					Type:     "unknown",
					Nullable: true,
				}
			}
		}
	default:
		// Fallback: use the text content
		if text := targetEl.GetText(); text != "" {
			return &core.Column{
				Name:     fw.dialect.NormalizeIdentifier(text),
				Type:     "unknown",
				Nullable: true,
			}
		}
	}

	return nil
}

// extractColumnNameFromColumnRef extracts the column name from a column reference

func (fw *fromWalker) extractColumnNameFromColumnRef(columnRef pg.IColumnrefContext) string {
	if columnRef == nil {
		return ""
	}

	// First try to get the full text of the column reference
	fullText := columnRef.GetText()
	if strings.Contains(fullText, ".") {
		// Extract the column name (last part after dot)
		parts := strings.Split(fullText, ".")
		if len(parts) > 1 {
			return fw.dialect.NormalizeIdentifier(strings.TrimSpace(parts[len(parts)-1]))
		}
	}

	// Fallback: get the column ID directly
	if colId := columnRef.Colid(); colId != nil {
		return fw.dialect.NormalizeIdentifier(colId.GetText())
	}

	return ""
}

// extractAliasFromTargetLabel extracts the alias from a target label

func (fw *fromWalker) extractAliasFromTargetLabel(targetLabel *pg.Target_labelContext) string {
	if targetLabel == nil {
		return ""
	}

	// Get the label text
	if alias := targetLabel.Target_alias(); alias != nil {
		aliasText := alias.GetText()
		// Remove "AS" prefix if present (case insensitive)
		aliasText = strings.TrimSpace(aliasText)
		if len(aliasText) > 2 && strings.ToUpper(aliasText[:2]) == "AS" {
			aliasText = aliasText[2:]
		}
		// Trim whitespace again after removing AS
		aliasText = strings.TrimSpace(aliasText)
		return fw.dialect.NormalizeIdentifier(aliasText)
	}

	return ""
}

// extractColumnRefFromExpression extracts a column reference from an expression

func (fw *fromWalker) extractColumnRefFromExpression(expr pg.IA_exprContext) pg.IColumnrefContext {
	if expr == nil {
		return nil
	}

	// For now, return nil as creating proper column reference contexts is complex
	// We'll handle this differently in the calling code
	return nil
}

func (fw *fromWalker) processSubqueryRelationRef(relationRef pg.ITable_refContext, defaultSchema string) *core.RelationRef {
	if relationRef == nil {
		return nil
	}

	// Get subquery alias and track position
	alias := ""
	availableFromPos := -1
	if aliasClause := relationRef.Opt_alias_clause(); aliasClause != nil {
		if tableAliasClause := aliasClause.Table_alias_clause(); tableAliasClause != nil {
			if tableAlias := tableAliasClause.Table_alias(); tableAlias != nil {
				alias = fw.dialect.NormalizeIdentifier(tableAlias.GetText())
				// Subquery becomes available right after the alias token
				if stop := tableAlias.GetStop(); stop != nil {
					availableFromPos = stop.GetTokenIndex() + 1
				}
			}
		}
	}

	// For subqueries, use the alias as the Table field if present
	// This matches the expected behavior in references_test.go
	tableName := alias
	if tableName == "" {
		tableName = "" // Keep empty if no alias
	}

	return &core.RelationRef{
		Schema:        "",        // Subqueries don't have schemas
		Table:         tableName, // Use alias as table name for subqueries
		Alias:         "",
		ScopeStartPos: availableFromPos,
		ScopeEndPos:   -1, // Available until end of query
	}
}

func (fw *fromWalker) processRelationRef(relationRef pg.ITable_refContext, defaultSchema string) *core.RelationRef {
	if relationRef == nil {
		return nil
	}

	relationExpr := relationRef.Relation_expr()
	if relationExpr == nil {
		return nil
	}

	qualifiedName := relationExpr.Qualified_name()
	if qualifiedName == nil {
		return nil
	}

	// Parse qualified name (schema.table)
	colId := qualifiedName.Colid()
	if colId == nil {
		return nil
	}

	var schema, table string
	var nameLine, nameCol, nameEndCol int
	if indirection := qualifiedName.Indirection(); indirection != nil {
		// This is schema.table format
		schema = fw.dialect.NormalizeIdentifier(colId.GetText())
		indirectionEls := indirection.AllIndirection_el()
		if len(indirectionEls) > 0 {
			if attrName := indirectionEls[0].Attr_name(); attrName != nil {
				table = fw.dialect.NormalizeIdentifier(attrName.GetText())
				if tok := attrName.GetStop(); tok != nil {
					nameLine = tok.GetLine()
					nameCol = tok.GetColumn()
					nameEndCol = tok.GetColumn() + len(tok.GetText())
				}
			}
		}
	} else {
		// This is just table name
		table = fw.dialect.NormalizeIdentifier(colId.GetText())
		schema = defaultSchema
		if tok := colId.GetStop(); tok != nil {
			nameLine = tok.GetLine()
			nameCol = tok.GetColumn()
			nameEndCol = tok.GetColumn() + len(tok.GetText())
		}
	}

	// Parse table alias and track position
	alias := ""
	availableFromPos := -1
	if aliasClause := relationRef.Opt_alias_clause(); aliasClause != nil {
		if tableAliasClause := aliasClause.Table_alias_clause(); tableAliasClause != nil {
			if tableAlias := tableAliasClause.Table_alias(); tableAlias != nil {
				normalized := fw.dialect.NormalizeIdentifier(tableAlias.GetText())
				// Filter out reserved keywords (keyword pollution fix)
				// Keywords are typically stored in uppercase, so check both normalized and uppercase
				reservedKeywords := fw.dialect.GetReservedKeywords()
				aliasUpper := strings.ToUpper(normalized)
				if reservedKeywords == nil || (!reservedKeywords[normalized] && !reservedKeywords[aliasUpper]) {
					alias = normalized
					// Table becomes available right after the alias token
					if stop := tableAlias.GetStop(); stop != nil {
						availableFromPos = stop.GetTokenIndex() + 1
					}
				}
				// If it's a keyword, leave alias as empty string and don't set availableFromPos
			}
		}
	} else {
		// If no alias, available right after the table name
		if stop := relationRef.GetStop(); stop != nil {
			availableFromPos = stop.GetTokenIndex() + 1
		}
	}

	return &core.RelationRef{
		Schema:        schema,
		Table:         table,
		Alias:         alias,
		ScopeStartPos: availableFromPos,
		ScopeEndPos:   -1, // Available until end of query (will be refined for subqueries)
		Line:          nameLine,
		Col:           nameCol,
		EndCol:        nameEndCol,
	}
}

// columnRefQualifiersAndField parses columnref: colid followed by optional indirection (.attr_name)+.
// The last attribute is the column name; preceding segments (colid + prior attrs) are qualifiers.
func (d *Dialect) columnRefQualifiersAndField(colRef pg.IColumnrefContext) (qualifiers []string, fieldName string, ok bool) {
	if colRef == nil {
		return nil, "", false
	}
	colid := colRef.Colid()
	if colid == nil {
		return nil, "", false
	}
	first := d.colidSegmentName(colid)
	if first == "" {
		return nil, "", false
	}

	ind := colRef.Indirection()
	if ind == nil {
		return nil, first, true
	}

	var attrNames []string
	for _, el := range ind.AllIndirection_el() {
		if el == nil || el.DOT() == nil {
			continue
		}
		an := el.Attr_name()
		if an == nil {
			continue
		}
		cl := an.Collabel()
		if cl == nil {
			continue
		}
		seg := d.collabelSegmentName(cl)
		if seg == "" {
			continue
		}
		attrNames = append(attrNames, seg)
	}
	if len(attrNames) == 0 {
		return nil, first, true
	}

	qualifiers = append([]string{first}, attrNames[:len(attrNames)-1]...)
	fieldName = attrNames[len(attrNames)-1]
	return qualifiers, fieldName, true
}

func (d *Dialect) colidSegmentName(colid pg.IColidContext) string {
	if colid == nil {
		return ""
	}
	if id := colid.Identifier(); id != nil {
		return d.NormalizeIdentifier(id.GetText())
	}
	if uk := colid.Unreserved_keyword(); uk != nil {
		return d.NormalizeIdentifier(uk.GetText())
	}
	if cn := colid.Col_name_keyword(); cn != nil {
		return d.NormalizeIdentifier(cn.GetText())
	}
	if pk := colid.Plsql_unreserved_keyword(); pk != nil {
		return d.NormalizeIdentifier(pk.GetText())
	}
	if colid.LEFT() != nil {
		return d.NormalizeIdentifier(colid.LEFT().GetText())
	}
	if colid.RIGHT() != nil {
		return d.NormalizeIdentifier(colid.RIGHT().GetText())
	}
	return d.NormalizeIdentifier(strings.TrimSpace(colid.GetText()))
}

func (d *Dialect) collabelSegmentName(cl pg.ICollabelContext) string {
	if cl == nil {
		return ""
	}
	if id := cl.Identifier(); id != nil {
		return d.NormalizeIdentifier(id.GetText())
	}
	if uk := cl.Unreserved_keyword(); uk != nil {
		return d.NormalizeIdentifier(uk.GetText())
	}
	if cn := cl.Col_name_keyword(); cn != nil {
		return d.NormalizeIdentifier(cn.GetText())
	}
	if tf := cl.Type_func_name_keyword(); tf != nil {
		return d.NormalizeIdentifier(tf.GetText())
	}
	if rk := cl.Reserved_keyword(); rk != nil {
		return d.NormalizeIdentifier(rk.GetText())
	}
	if pk := cl.Plsql_unreserved_keyword(); pk != nil {
		return d.NormalizeIdentifier(pk.GetText())
	}
	return d.NormalizeIdentifier(strings.TrimSpace(cl.GetText()))
}

func (d *Dialect) processColumnRef(
	colRef pg.IColumnrefContext,
	relationRefs []core.RelationRef,
	meta core.Metadata,
) []core.Column {
	if colRef == nil {
		return nil
	}
	qualifiers, fieldName, ok := d.columnRefQualifiersAndField(colRef)
	if !ok || fieldName == "" {
		return nil
	}
	return core.ResolveColumnFromRelationRefs(qualifiers, fieldName, relationRefs, meta, d)
}
