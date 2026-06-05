package db_client

import (
	"fmt"
	"strings"

	"selectDb/backend/graph"
	"selectDb/backend/src/sqllang"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)


func (dbc *DbClient) inspectStatement(
	statement string,
	dbInstance *graph.DBInstanceNode,
) (*core.Metadata, []core.InspectStatement, bool) {
	metadata, err := dbc.getCachedMetadata(dbInstance, false)
	if err != nil {
		return nil, nil, false
	}

	dialect := engine.GetDialect(dbInstance.DBType)
	inspectStatements := engine.Inspect(dialect, metadata, statement)
	if len(inspectStatements) == 0 {
		return nil, nil, false
	}

	return metadata, inspectStatements, true
}

// InspectStatement satisfies sqllang.InspectFunc.
// TODO: move to dialect/ when metadata cache is shared with remote backend.
func (dbc *DbClient) InspectStatement(statement string, dbInstance *graph.DBInstanceNode) []core.InspectStatement {
	_, inspectStatements, _ := dbc.inspectStatement(statement, dbInstance)
	return inspectStatements
}

// buildSchemaTableMaps returns two O(1) lookup maps: schema name → schema, "schema.table" → table.
func buildSchemaTableMaps(metadata *core.Metadata) (map[string]*core.Schema, map[string]*core.Table) {
	schemaMap := make(map[string]*core.Schema)
	tableMap := make(map[string]*core.Table)

	for i := range metadata.Schemas {
		schema := &metadata.Schemas[i]
		schemaKey := strings.ToLower(schema.Name)
		schemaMap[schemaKey] = schema

		for j := range schema.Tables {
			table := &schema.Tables[j]
			tableKey := strings.ToLower(schema.Name + "." + table.Name)
			tableMap[tableKey] = table
		}
	}

	return schemaMap, tableMap
}

// findTable looks up schema.table with a nil-caching hot path to avoid re-walking metadata.
func findTable(
	tableMap map[string]*core.Table,
	cache map[string]*core.Table,
	schemaName, tableName string,
) *core.Table {
	tableKey := schemaName + "." + tableName
	cacheKey := strings.ToLower(tableKey)

	if cached, exists := cache[cacheKey]; exists {
		return cached
	}
	if table, exists := tableMap[strings.ToLower(tableKey)]; exists {
		cache[cacheKey] = table
		return table
	}
	cache[cacheKey] = nil
	return nil
}

// columnEditMetaToGraph maps engine.ColumnEditMeta to the frontend wire shape.
// Conversion stays in the app so dialect/ never imports graph types.
func columnEditMetaToGraph(in []engine.ColumnEditMeta) []graph.ColumnMetadata {
	out := make([]graph.ColumnMetadata, len(in))
	for i, m := range in {
		out[i] = graph.ColumnMetadata{
			HasAllPrimaryKeys:  m.HasAllPrimaryKeys,
			IsPrimaryKey:       m.IsPrimaryKey,
			IsForeignKey:       m.IsForeignKey,
			DatabaseID:         m.DatabaseID,
			Schema:             m.Schema,
			Table:              m.Table,
			OriginalColumnName: m.OriginalColumnName,
			PrimaryKeys:        m.PrimaryKeys,
			PrimaryKeysIdxs:    m.PrimaryKeysIdxs,
			DataType:           m.DataType,
			EnumValues:         m.EnumValues,
		}
	}
	return out
}

// Query
// varResolver wraps fallback with runtime var overrides checked first.
func varResolver(runtimeVars map[string]string, fallback sqllang.VarReplacer) sqllang.VarReplacer {
	if len(runtimeVars) == 0 {
		return fallback
	}
	return sqllang.WithOverrides(runtimeVars, fallback)
}

// queryKey matches the frontend loading key format.
func queryKey(dbInstanceID, fileID string) string {
	return fmt.Sprintf("db:%s:file:%s", dbInstanceID, fileID)
}
