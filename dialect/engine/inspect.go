package engine

import (
	"strings"

	"github.com/selectDb/dialect/core"
)

// Inspect parses sql against meta and returns one InspectStatement per
// top-level statement. Returns nil when dialect or meta is nil, or when
// the dialect does not support inspection.
func Inspect(dialect core.SQLDialect, meta *core.Metadata, sql string) []core.InspectStatement {
	if dialect == nil || meta == nil {
		return nil
	}
	return dialect.Inspect(*meta, sql)
}

// FirstSelectStatement returns the first SELECT in stmts. Convenience for
// callers that want editable-column analysis only on read queries.
func FirstSelectStatement(stmts []core.InspectStatement) (core.InspectStatement, bool) {
	for _, r := range stmts {
		if r.Operation == core.InspectOpSelect {
			return r, true
		}
	}
	return core.InspectStatement{}, false
}

// AnalyzeEditableColumns determines which result columns are editable.
// A column is editable when it comes from a real table, the table has a
// primary key, and every primary-key column appears in the SELECT.
//
// Result length matches len(stmt.Fields). The mapping from Field index to
// driver column position is approximate for non-trivial queries. Derived
// expressions, multi-arg functions, and SELECT * with stale metadata can
// drift. Callers that need exact driver-position alignment should compare
// names against rows.Columns().
func AnalyzeEditableColumns(meta *core.Metadata, stmt core.InspectStatement, instanceID string) []ColumnEditMeta {
	if meta == nil || len(stmt.Fields) == 0 {
		return nil
	}
	fields := stmt.Fields
	_, tableMap := buildSchemaTableMaps(meta)
	tableCache := make(map[string]*core.Table)

	out := make([]ColumnEditMeta, len(fields))

	tableToColumns := make(map[string][]int)
	tableLookupByName := make(map[string]*core.Table)

	for colIndex, field := range fields {
		if field.Table == "" || field.Schema == "" {
			continue
		}

		tableKey := field.Schema + "." + field.Table
		foundTable := findTable(tableMap, tableCache, field.Schema, field.Table)
		if foundTable == nil {
			continue
		}

		tableLookupByName[tableKey] = foundTable

		isPrimaryKey := false
		for _, pkCol := range foundTable.PrimaryKey {
			if strings.EqualFold(pkCol, field.Name) {
				isPrimaryKey = true
				break
			}
		}
		isForeignKey := false
		dataType := ""
		var enumValues []string
		for _, col := range foundTable.Columns {
			if strings.EqualFold(col.Name, field.Name) {
				isForeignKey = col.IsForeignKey
				dataType = col.Type
				enumValues = col.EnumValues
				break
			}
		}

		out[colIndex] = ColumnEditMeta{
			IsPrimaryKey:       isPrimaryKey,
			IsForeignKey:       isForeignKey,
			DatabaseID:         instanceID,
			Schema:             field.Schema,
			Table:              field.Table,
			OriginalColumnName: field.Name,
			PrimaryKeys:        foundTable.PrimaryKey,
			DataType:           dataType,
			EnumValues:         enumValues,
		}

		tableToColumns[tableKey] = append(tableToColumns[tableKey], colIndex)
	}

	for tableKey, columnIndices := range tableToColumns {
		foundTable := tableLookupByName[tableKey]
		if foundTable == nil || len(foundTable.PrimaryKey) == 0 {
			continue
		}

		primaryKeysIdxs, hasAllPrimaryKeys := findPrimaryKeyIndices(
			foundTable.PrimaryKey,
			columnIndices,
			fields,
		)

		for _, colIdx := range columnIndices {
			out[colIdx].HasAllPrimaryKeys = hasAllPrimaryKeys
			out[colIdx].PrimaryKeysIdxs = primaryKeysIdxs
		}
	}

	return out
}

// buildSchemaTableMaps returns (schemaName -> *Schema, "schema.table" -> *Table)
// lookup maps for O(1) access during inspection.
func buildSchemaTableMaps(meta *core.Metadata) (map[string]*core.Schema, map[string]*core.Table) {
	schemaMap := make(map[string]*core.Schema)
	tableMap := make(map[string]*core.Table)

	for i := range meta.Schemas {
		schema := &meta.Schemas[i]
		schemaMap[strings.ToLower(schema.Name)] = schema

		for j := range schema.Tables {
			table := &schema.Tables[j]
			tableMap[strings.ToLower(schema.Name+"."+table.Name)] = table
		}
	}

	return schemaMap, tableMap
}

// findTable looks up a table by (schema, table). cache stores nil for misses
// so we don't re-walk the metadata for the same key.
func findTable(
	tableMap map[string]*core.Table,
	cache map[string]*core.Table,
	schemaName, tableName string,
) *core.Table {
	cacheKey := strings.ToLower(schemaName + "." + tableName)
	if cached, ok := cache[cacheKey]; ok {
		return cached
	}
	if table, ok := tableMap[cacheKey]; ok {
		cache[cacheKey] = table
		return table
	}
	cache[cacheKey] = nil
	return nil
}

// findPrimaryKeyIndices verifies every primary-key column appears in fields
// (matched by InspectField.Name, not alias). Returns the indices in fields
// where each PK column was found and whether all PK columns were present.
func findPrimaryKeyIndices(
	primaryKeyNames []string,
	columnIndices []int,
	fields []core.InspectField,
) ([]int, bool) {
	primaryKeysIdxs := make([]int, 0)
	foundPKs := make(map[string]bool)

	for _, pkColName := range primaryKeyNames {
		pkFound := false
		for _, colIdx := range columnIndices {
			field := fields[colIdx]
			if strings.EqualFold(field.Name, pkColName) {
				if !foundPKs[pkColName] {
					primaryKeysIdxs = append(primaryKeysIdxs, colIdx)
					foundPKs[pkColName] = true
					pkFound = true
					break
				}
			}
		}
		if !pkFound {
			return nil, false
		}
	}

	return primaryKeysIdxs, len(primaryKeysIdxs) == len(primaryKeyNames)
}
