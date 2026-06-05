package core

import (
	"fmt"
	"strings"

)

// CandidateType represents the type of a completion suggestion
type CandidateType int

const (
	CandidateTypeKeyword CandidateType = iota + 1
	CandidateTypeSchema
	CandidateTypeTable
	CandidateTypeForeignTable
	CandidateTypeView
	CandidateTypeMaterializedView
	CandidateTypeColumn
	CandidateTypeOperator
	CandidateTypeType
	CandidateTypeFunction
	CandidateTypeEnumValue
	CandidateTypeSetting
)

// Candidate is a completion suggestion item
type Candidate struct {
	Type       CandidateType
	Text       string     // Display text shown in completion list
	InsertText string     // Text to insert (may contain snippet placeholders like $1, $0)
	Definition string
	Comment    string
}

// CompletionTarget is a bitmask indicating which database object types should be shown
type CompletionTarget int

const (
	CompletionTargetSchema CompletionTarget = 1 << iota
	CompletionTargetTable
	CompletionTargetColumn
	completionTargetRefOnlyFlag // Internal flag: ref-only mode
	completionTargetAllFlag     // Internal flag: all mode
	CompletionTargetOperator    // Operator completion (=, <>, LIKE, etc.)
	CompletionTargetEnumValue   // Enum value completion (col = '|', col IN ('|'))
	CompletionTargetSetting     // Runtime parameter completion (@@var, SHOW, PRAGMA)

	CompletionTargetSchemaAndTable        = CompletionTargetSchema | CompletionTargetTable
	CompletionTargetSchemaAndRelationRefOnly = CompletionTargetSchema | CompletionTargetTable | completionTargetRefOnlyFlag
	CompletionTargetSchemaAndTableAll     = CompletionTargetSchema | CompletionTargetTable | completionTargetAllFlag
	CompletionTargetTableAndColumn        = CompletionTargetTable | CompletionTargetColumn
	CompletionTargetAll                   = CompletionTargetSchema | CompletionTargetTable | CompletionTargetColumn
)

// PrecedingColumnInfo holds info about the column preceding the caret (for operator completion)
type PrecedingColumnInfo struct {
	Name string
	Type string
}

// CompletionContext represents a parsed completion context
type CompletionContext struct {
	Parts             []string             // Qualified parts: ["schema", "table"] or ["table"]
	CaretAfterDot     bool                 // True if caret is after a dot
	Targets           CompletionTarget     // What to complete (schema/table/column)
	SchemaFilter      string               // Schema from qualified parts
	TargetTable       string               // Table from qualified parts
	KeywordContext    CompletionTarget     // SQL keyword context (SELECT/FROM/JOIN)
	PrecedingColumn   *PrecedingColumnInfo // Column before caret (for operator/enum-value completion in WHERE)
	InsertTargetTable string               // Table name from INSERT INTO <table> (for column list completion)
	ValuePosition     bool                 // Caret is in a value slot after an enum column (col = '|', col IN ('|'))
}

type CompletionStrategy struct {
	dialect SQLDialect
}

// NewCompletionStrategy creates a new completion strategy with the rationalized algorithm
func NewCompletionStrategy(d SQLDialect) *CompletionStrategy {
	return &CompletionStrategy{dialect: d}
}

func (cs *CompletionStrategy) CompleteFromSQL(
	ctx CompletionContext,
	sql string,
	caretLine int,
	caretCol int,
	meta Metadata,
	refs []RelationRef,
	cteTables []RelationRef,
	columnAliases []ColumnAlias,
	reservedKeywords map[string]bool,
) []Candidate {
	if ctx.Targets&CompletionTargetSetting != 0 {
		return cs.completeSettings(meta)
	}

	caretOffset := charOffsetFromLineCol(sql, caretLine, caretCol)
	caretQuoted := caretOffset > 0 && caretOffset < len(sql) && sql[caretOffset] == '"'
	nestingLevel := countParenNesting(sql, caretOffset)

	inScopeRefs := filterByCharScope(refs, caretOffset, nestingLevel)
	inScopeCtes := filterByCharScope(cteTables, caretOffset, nestingLevel)

	var schemas, tables, views, columns, operators, enumValues []Candidate

	if ctx.Targets&CompletionTargetSchema != 0 {
		schemas = cs.completeSchemas(meta, caretQuoted, reservedKeywords)
	}

	if ctx.Targets&CompletionTargetTable != 0 {
		keywordIsAllMode := (ctx.KeywordContext & completionTargetAllFlag) != 0
		isAllMode := keywordIsAllMode || (ctx.Targets == CompletionTargetAll && len(inScopeRefs) == 0 && len(inScopeCtes) == 0)

		if ctx.SchemaFilter != "" {
			if !isAllMode && len(inScopeRefs) > 0 {
				normalize := cs.dialect.NormalizeIdentifier
				normalizedSchema := normalize(ctx.SchemaFilter)
				seen := map[string]bool{}
				for _, r := range inScopeRefs {
					refSchema := normalize(r.Schema)
					if refSchema == normalizedSchema || (refSchema == "" && normalizedSchema == normalize(GetDefaultSchema(meta))) {
						name := r.Alias
						if name == "" {
							name = r.Table
						}
						if name != "" && !seen[name] {
							tables = append(tables, Candidate{Type: CandidateTypeTable, Text: cs.dialect.QuoteIdentifierIfNeeded(name, caretQuoted, reservedKeywords)})
							seen[name] = true
						}
					}
				}
				if len(tables) > 0 {
					goto doneWithTables
				}
			}
			schemasToSearch := map[string]bool{ctx.SchemaFilter: true}
			tables, views = ListRelations(meta, schemasToSearch, cs.dialect.QuoteIdentifierIfNeeded, caretQuoted, reservedKeywords)
			goto doneWithTables
		}

		// No schema filter
		{
			seen := map[string]bool{}
			addTable := func(name string) {
				if name != "" && !seen[name] {
					tables = append(tables, Candidate{Type: CandidateTypeTable, Text: cs.dialect.QuoteIdentifierIfNeeded(name, caretQuoted, reservedKeywords)})
					seen[name] = true
				}
			}

			for _, r := range inScopeRefs {
				name := r.Alias
				if name == "" {
					name = r.Table
				}
				addTable(name)
			}
			if !isAllMode && len(inScopeRefs) > 0 {
				goto addCtes
			}

			for _, vt := range inScopeCtes {
				addTable(vt.Table)
			}
			if !isAllMode && len(inScopeCtes) > 0 {
				goto doneWithTables
			}

			if isAllMode {
				schemasToSearch := map[string]bool{GetDefaultSchema(meta): true, meta.CurrentSchema: true, "": true}
				metaTables, metaViews := ListRelations(meta, schemasToSearch, cs.dialect.QuoteIdentifierIfNeeded, caretQuoted, reservedKeywords)
				for _, t := range metaTables {
					normalized := cs.dialect.NormalizeIdentifier(t.Text)
					if !seen[normalized] {
						tables = append(tables, t)
						seen[normalized] = true
					}
				}
				views = append(views, metaViews...)
			}
		}
	addCtes:
		{
			seen := map[string]bool{}
			for _, t := range tables {
				seen[cs.dialect.NormalizeIdentifier(t.Text)] = true
			}
			for _, vt := range inScopeCtes {
				name := vt.Table
				if name != "" && !seen[cs.dialect.NormalizeIdentifier(name)] {
					tables = append(tables, Candidate{Type: CandidateTypeTable, Text: cs.dialect.QuoteIdentifierIfNeeded(name, caretQuoted, reservedKeywords)})
				}
			}
		}
	doneWithTables:
	}

	if ctx.Targets&CompletionTargetColumn != 0 {
		if ctx.InsertTargetTable != "" {
			columns = cs.completeColumnsForTable(ctx.InsertTargetTable, inScopeCtes, inScopeRefs, inScopeRefs, meta, caretQuoted, reservedKeywords)
		} else if ctx.TargetTable != "" {
			columns = cs.completeColumnsForTable(ctx.TargetTable, inScopeCtes, inScopeRefs, inScopeRefs, meta, caretQuoted, reservedKeywords)
		} else {
			columns = cs.completeColumnsUnqualified(inScopeRefs, inScopeCtes, meta, caretQuoted, reservedKeywords)
		}
	}

	if ctx.Targets&CompletionTargetOperator != 0 && ctx.PrecedingColumn != nil {
		operators = cs.completeOperators(ctx, meta, inScopeRefs, inScopeCtes)
	}

	if ctx.Targets&CompletionTargetEnumValue != 0 && ctx.PrecedingColumn != nil {
		enumValues = cs.completeEnumValues(ctx, meta, inScopeRefs, inScopeCtes)
	}

	var all []Candidate
	all = append(all, schemas...)
	all = append(all, tables...)
	all = append(all, views...)
	all = append(all, columns...)
	all = append(all, operators...)
	all = append(all, enumValues...)
	return all
}

func charOffsetFromLineCol(sql string, line, col int) int {
	currentLine := 1
	for i, ch := range sql {
		if currentLine == line {
			return i + col
		}
		if ch == '\n' {
			currentLine++
		}
	}
	return len(sql)
}

func countParenNesting(sql string, offset int) int {
	level := 0
	for i := 0; i < len(sql) && i < offset; i++ {
		if sql[i] == '(' {
			level++
		} else if sql[i] == ')' && level > 0 {
			level--
		}
	}
	return level
}

func filterByCharScope(refs []RelationRef, caretOffset, maxNesting int) []RelationRef {
	var out []RelationRef
	for _, ref := range refs {
		if ScopeContainsCaret(ref.ScopeStartPos, ref.ScopeEndPos, caretOffset) && ref.NestingLevel <= maxNesting {
			out = append(out, ref)
		}
	}
	return out
}

func (cs *CompletionStrategy) completeSettings(meta Metadata) []Candidate {
	var out []Candidate
	for _, s := range meta.Schemas {
		for _, st := range s.Settings {
			out = append(out, Candidate{
				Type:       CandidateTypeSetting,
				Text:       st.Name,
				InsertText: st.Name,
				Comment:    st.Description,
				Definition: st.Value,
			})
		}
	}
	return out
}

func (cs *CompletionStrategy) completeSchemas(
	meta Metadata,
	caretQuoted bool,
	reservedKeywords map[string]bool,
) []Candidate {
	var schemas []Candidate
	for _, schemaName := range ListAllSchemas(meta) {
		schemas = append(schemas, Candidate{
			Type: CandidateTypeSchema,
			Text: cs.dialect.QuoteIdentifierIfNeeded(schemaName, caretQuoted, reservedKeywords),
		})
	}
	return schemas
}

func (cs *CompletionStrategy) completeOperators(
	ctx CompletionContext,
	meta Metadata,
	refs []RelationRef,
	cteTables []RelationRef,
) []Candidate {
	if ctx.PrecedingColumn == nil {
		return nil
	}

	// Resolve the column type from refs/metadata
	columnType := cs.resolveColumnType(ctx.PrecedingColumn.Name, meta, refs, cteTables)

	// Get operators for this column type from the dialect
	operators := cs.dialect.GetOperatorsForType(columnType)

	var candidates []Candidate
	for _, op := range operators {
		candidates = append(candidates, Candidate{
			Type:       CandidateTypeOperator,
			Text:       op.Text,
			InsertText: op.InsertText,
			Definition: columnType,
			Comment:    op.Description,
		})
	}
	return candidates
}

// resolveColumnType looks up the column type from metadata or refs
func (cs *CompletionStrategy) resolveColumnType(
	columnName string,
	meta Metadata,
	refs []RelationRef,
	cteTables []RelationRef,
) string {
	if col, ok := cs.resolveColumn(columnName, meta, refs, cteTables); ok {
		return col.Type
	}
	return "unknown"
}

// resolveColumn resolves a (possibly table-qualified) column reference to its
// core.Column using the same search order as resolveColumnType: in-scope CTEs,
// then table refs, then any table for an unqualified name. EnumValues on the
// returned column is already populated by EnrichEnumValues.
func (cs *CompletionStrategy) resolveColumn(
	columnName string,
	meta Metadata,
	refs []RelationRef,
	cteTables []RelationRef,
) (Column, bool) {
	normalize := cs.dialect.NormalizeIdentifier

	var tableName, colName string
	if parts := strings.Split(columnName, "."); len(parts) == 2 {
		tableName = parts[0]
		colName = parts[1]
	} else {
		colName = columnName
	}

	normalizedCol := normalize(colName)
	normalizedTable := ""
	if tableName != "" {
		normalizedTable = normalize(tableName)
	}

	for _, cte := range cteTables {
		if normalizedTable != "" && normalize(cte.Table) != normalizedTable {
			continue
		}
		for _, col := range cte.Columns {
			if normalize(col.Name) == normalizedCol {
				return col, true
			}
		}
	}

	for _, ref := range refs {
		refKey := ref.Alias
		if refKey == "" {
			refKey = ref.Table
		}
		if normalizedTable != "" && normalize(refKey) != normalizedTable {
			continue
		}
		cols := GetColumnsForTable(meta, ref.Schema, ref.Table, normalize)
		for _, col := range cols {
			if normalize(col.Name) == normalizedCol {
				return col, true
			}
		}
	}

	if normalizedTable == "" {
		for _, schema := range meta.Schemas {
			for _, table := range GetAllTablesFromSchema(schema) {
				for _, col := range table.Columns {
					if normalize(col.Name) == normalizedCol {
						return col, true
					}
				}
			}
		}
	}

	return Column{}, false
}

// completeEnumValues suggests the allowed labels of an enum column when the
// caret sits in a value slot (col = '|' or col IN ('|')). Returns nil unless
// the preceding column resolves to a column with EnumValues.
func (cs *CompletionStrategy) completeEnumValues(
	ctx CompletionContext,
	meta Metadata,
	refs []RelationRef,
	cteTables []RelationRef,
) []Candidate {
	if ctx.PrecedingColumn == nil {
		return nil
	}
	col, ok := cs.resolveColumn(ctx.PrecedingColumn.Name, meta, refs, cteTables)
	if !ok || len(col.EnumValues) == 0 {
		return nil
	}
	candidates := make([]Candidate, 0, len(col.EnumValues))
	for _, v := range col.EnumValues {
		candidates = append(candidates, Candidate{
			Type:       CandidateTypeEnumValue,
			Text:       v,
			InsertText: "'" + strings.ReplaceAll(v, "'", "''") + "'",
			Definition: col.Type,
		})
	}
	return candidates
}

// completeColumnsForTable completes columns for a specific table
// Searches in order: filtered CTEs -> aliases -> metadata
func (cs *CompletionStrategy) completeColumnsForTable(
	targetTable string,
	filteredCteTables []RelationRef,
	filteredRefs []RelationRef,
	allRefs []RelationRef,
	meta Metadata,
	caretQuoted bool,
	reservedKeywords map[string]bool,
) []Candidate {
	normalize := cs.dialect.NormalizeIdentifier
	targetNormalized := normalize(targetTable)
	var columns []Candidate

	// 1. First check if target matches a RelationRef (CTE or subquery) directly
	for _, cte := range filteredCteTables {
		if normalize(cte.Table) == targetNormalized {
			for _, col := range cte.Columns {
				columns = append(columns, cs.buildColumnCandidate(col, "", targetTable, nil, caretQuoted, reservedKeywords))
			}
			return columns
		}
	}

	// 2. Check aliases and table names in filtered refs, then all refs
	for _, refSet := range [][]RelationRef{filteredRefs, allRefs} {
		for _, ref := range refSet {
			// Check if target matches alias or table name
			refKey := ref.Alias
			if refKey == "" {
				refKey = ref.Table
			}
			refKeyNormalized := normalize(refKey)
			if refKeyNormalized == targetNormalized {
				// Check if this ref points to a CTE/subquery
				for _, cte := range filteredCteTables {
					if normalize(cte.Table) == normalize(ref.Table) || normalize(cte.Table) == normalize(ref.Alias) {
						for _, col := range cte.Columns {
							columns = append(columns, cs.buildColumnCandidate(col, "", targetTable, nil, caretQuoted, reservedKeywords))
						}
						return columns
					}
				}
				// Use columns from metadata - use ref.Table (actual table) not the alias
				// First try the ref's schema with normalized comparison
				cols := GetColumnsForTable(meta, ref.Schema, ref.Table, normalize)
				if len(cols) > 0 {
					for _, col := range cols {
						columns = append(columns, cs.buildColumnCandidate(col, "", targetTable, nil, caretQuoted, reservedKeywords))
					}
					return columns
				}
				// If not found in ref's schema, search all schemas for the table
				// This handles cases where table is referenced without schema but exists in a non-default schema
				for _, schema := range meta.Schemas {
					cols = GetColumnsForTable(meta, schema.Name, ref.Table, normalize)
					if len(cols) > 0 {
						for _, col := range cols {
							columns = append(columns, cs.buildColumnCandidate(col, "", targetTable, nil, caretQuoted, reservedKeywords))
						}
						return columns
					}
				}
				// No columns found for this ref, continue to next ref
			}
		}
	}

	// 2. Check metadata - try default schema first
	defaultSchema := GetDefaultSchema(meta)
	cols := GetColumnsForTable(meta, defaultSchema, targetTable, normalize)
	if len(cols) > 0 {
		for _, col := range cols {
			columns = append(columns, cs.buildColumnCandidate(col, defaultSchema, targetTable, nil, caretQuoted, reservedKeywords))
		}
		return columns
	}

	// 3. Try all schemas
	for _, schema := range meta.Schemas {
		cols := GetColumnsForTable(meta, schema.Name, targetTable, normalize)
		if len(cols) > 0 {
			for _, col := range cols {
				columns = append(columns, cs.buildColumnCandidate(col, schema.Name, targetTable, nil, caretQuoted, reservedKeywords))
			}
			return columns
		}
	}

	return columns
}

// completeColumnsUnqualified completes columns when no specific table is targeted
// Shows columns from all in-scope tables with ambiguity detection
func (cs *CompletionStrategy) completeColumnsUnqualified(
	filteredRefs []RelationRef,
	filteredCteTables []RelationRef,
	meta Metadata,
	caretQuoted bool,
	reservedKeywords map[string]bool,
) []Candidate {
	// Build table info map and collect all tables
	tableInfoMap := make(map[string]TableInfo)
	allTableKeys := make(map[string]bool)

	for _, ref := range filteredRefs {
		tableKey := ref.Alias
		if tableKey == "" {
			tableKey = ref.Table
		}
		if tableKey != "" {
			tableInfoMap[tableKey] = TableInfo{
				OriginalSchema: ref.Schema,
				OriginalTable:  ref.Table,
				IsAlias:        ref.Alias != "",
			}
			allTableKeys[tableKey] = true
		}
	}
	for _, cte := range filteredCteTables {
		tableInfoMap[cte.Table] = TableInfo{
			OriginalSchema: "",
			OriginalTable:  cte.Table,
			IsAlias:        true,
		}
		allTableKeys[cte.Table] = true
	}

	if len(allTableKeys) == 0 {
		return []Candidate{}
	}

	// Build CTE lookup map for efficient access
	cteMap := make(map[string]*RelationRef)
	for i := range filteredCteTables {
		cteMap[filteredCteTables[i].Table] = &filteredCteTables[i]
	}

	// Count column occurrences for ambiguity detection
	shouldCheckAmbiguity := len(allTableKeys) > 1
	columnOccurrencesByName := make(map[string]int)
	if shouldCheckAmbiguity {
		for _, ref := range filteredRefs {
			cols := cs.getColumnsForRef(ref, cteMap, meta)
			for _, col := range cols {
				columnOccurrencesByName[col.Name]++
			}
		}
		for _, cte := range filteredCteTables {
			for _, col := range cte.Columns {
				columnOccurrencesByName[col.Name]++
			}
		}
	}

	// Build column candidates
	var prefixedColumns, simpleColumns []Candidate

	// Process all refs
	for _, ref := range filteredRefs {
		tableKey := cs.getTableKey(ref)
		if tableKey == "" {
			continue
		}

		cols := cs.getColumnsForRef(ref, cteMap, meta)
		schemaForDef := cs.getSchemaForDef(ref, tableKey, tableInfoMap)

		cs.appendColumnCandidates(cols, tableKey, schemaForDef, tableInfoMap, columnOccurrencesByName,
			shouldCheckAmbiguity, caretQuoted, reservedKeywords, &prefixedColumns, &simpleColumns)
	}

	// Process CTEs that aren't referenced by refs
	refTableSet := make(map[string]bool)
	for _, ref := range filteredRefs {
		refTableSet[ref.Table] = true
	}
	for _, cte := range filteredCteTables {
		if !refTableSet[cte.Table] {
			cs.appendColumnCandidates(cte.Columns, cte.Table, "", tableInfoMap, columnOccurrencesByName,
				shouldCheckAmbiguity, caretQuoted, reservedKeywords, &prefixedColumns, &simpleColumns)
		}
	}

	// Return prefixed (ambiguous) columns first, then simple ones
	var columns []Candidate
	columns = append(columns, prefixedColumns...)
	columns = append(columns, simpleColumns...)
	return columns
}

// getTableKey returns the display key for a reference (alias preferred, fallback to table)
func (cs *CompletionStrategy) getTableKey(ref RelationRef) string {
	if ref.Alias != "" {
		return ref.Alias
	}
	return ref.Table
}

// getColumnsForRef returns columns for a reference, checking CTEs first, then metadata
func (cs *CompletionStrategy) getColumnsForRef(
	ref RelationRef,
	cteMap map[string]*RelationRef,
	meta Metadata,
) []Column {
	if vt, ok := cteMap[ref.Table]; ok {
		return vt.Columns
	}
	return ListColumns(meta, ref.Schema, ref.Table)
}

// getSchemaForDef returns the schema to use in column definition for a reference
func (cs *CompletionStrategy) getSchemaForDef(
	ref RelationRef,
	tableKey string,
	tableInfoMap map[string]TableInfo,
) string {
	if tableInfo, hasInfo := tableInfoMap[tableKey]; hasInfo && tableInfo.IsAlias {
		return ""
	}
	return ref.Schema
}

// appendColumnCandidates appends column candidates, handling ambiguity detection
func (cs *CompletionStrategy) appendColumnCandidates(
	cols []Column,
	tableKey string,
	schemaForDef string,
	tableInfoMap map[string]TableInfo,
	columnOccurrencesByName map[string]int,
	shouldCheckAmbiguity bool,
	caretQuoted bool,
	reservedKeywords map[string]bool,
	prefixedColumns *[]Candidate,
	simpleColumns *[]Candidate,
) {
	for _, col := range cols {
		if shouldCheckAmbiguity && columnOccurrencesByName[col.Name] > 1 {
			*prefixedColumns = append(*prefixedColumns, cs.buildPrefixedColumnCandidate(col, tableKey, caretQuoted, reservedKeywords))
		} else {
			*simpleColumns = append(*simpleColumns, cs.buildColumnCandidate(col, schemaForDef, tableKey, tableInfoMap, caretQuoted, reservedKeywords))
		}
	}
}

// buildColumnCandidate creates a column candidate with proper definition
func (cs *CompletionStrategy) buildColumnCandidate(
	col Column,
	schemaName, tableName string,
	tableInfoMap map[string]TableInfo,
	caretQuoted bool,
	reservedKeywords map[string]bool,
) Candidate {
	def := cs.buildColumnDefinition(col, schemaName, tableName, tableInfoMap)

	return Candidate{
		Type:       CandidateTypeColumn,
		Text:       cs.dialect.QuoteIdentifierIfNeeded(col.Name, caretQuoted, reservedKeywords),
		Definition: def,
	}
}

// buildPrefixedColumnCandidate creates a column candidate with table prefix for ambiguous columns
func (cs *CompletionStrategy) buildPrefixedColumnCandidate(
	col Column,
	tableName string,
	caretQuoted bool,
	reservedKeywords map[string]bool,
) Candidate {
	// Create prefixed column name: table.column
	// Quote each part separately to avoid quoting the dot
	quotedTableName := cs.dialect.QuoteIdentifierIfNeeded(tableName, caretQuoted, reservedKeywords)
	quotedColumnName := cs.dialect.QuoteIdentifierIfNeeded(col.Name, caretQuoted, reservedKeywords)
	prefixedName := quotedTableName + "." + quotedColumnName

	// Create definition showing the table prefix
	def := fmt.Sprintf("%s | %s", tableName, col.Type)
	if !col.Nullable {
		def += ", NOT NULL"
	}

	return Candidate{
		Type:       CandidateTypeColumn,
		Text:       prefixedName,
		Definition: def,
	}
}

// buildColumnDefinition builds column definition string
func (cs *CompletionStrategy) buildColumnDefinition(
	col Column,
	schemaName, tableName string,
	tableInfoMap map[string]TableInfo,
) string {
	// Determine the display name for the table
	displayTable := tableName
	if tableInfo, hasInfo := tableInfoMap[tableName]; hasInfo {
		if tableInfo.IsAlias {
			// For aliases, show just the alias name
			displayTable = tableName
		} else {
			// For non-aliases, show schema.table if schema is available
			if schemaName != "" {
				displayTable = schemaName + "." + tableName
			} else {
				displayTable = tableName
			}
		}
	} else if schemaName != "" {
		// No table info, but we have schema - use it
		displayTable = schemaName + "." + tableName
	}

	// Build definition: "schema.table | type [NOT NULL]"
	def := fmt.Sprintf("%s | %s", displayTable, col.Type)
	if !col.Nullable {
		def += ", NOT NULL"
	}

	return def
}
