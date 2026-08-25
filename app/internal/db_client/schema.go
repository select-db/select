package db_client

import (
	"context"
	"fmt"
	"selectDb/internal/db/generated"
	fs_provider "selectDb/internal/fs_provider"
	"selectDb/internal/graph"
	"strconv"
	"strings"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
	"selectDb/internal/desktop"
)

type QuerySchemaParams struct {
	DatabaseInstanceID string
	NoCache            bool
}

func (dbc *DbClient) QuerySchema(queryParams QuerySchemaParams) error {
	dbInstance := dbc.Graph.GetDBInstanceNodeByID(queryParams.DatabaseInstanceID)
	if dbInstance == nil {
		return fmt.Errorf("could not find database instance with ID: %s", queryParams.DatabaseInstanceID)
	}

	// Get cached metadata (or fetch and cache it)
	// If NoCache is true, bypass cache and fetch fresh metadata.
	// Use the DB instance's SSH config (if any) so schema loading respects SSH tunneling and its timeouts.
	metadata, err := dbc.getCachedMetadata(dbInstance, queryParams.NoCache)
	if err != nil {
		return err
	}

	// Build schema nodes hierarchy: DB → Schemas → Schema → Tables, Views, Indexes, Triggers, Types, Functions (all direct children).
	var schemaNodes []*graph.DBInstanceItemNode
	dbName := dbInstance.Name
	for _, schema := range metadata.Schemas {
		schemaID := fmt.Sprintf("%s:schema:%s", dbInstance.ID, schema.Name)
		schemaPath := dbName + " / " + schema.Name

		// Convert all data to graph nodes for this schema
		dbIndexNodes, indexesByTable := convertIndexesToNodes(schema.Indexes, schemaID, schemaPath)
		dbTriggerNodes, triggersByTable := convertTriggersToNodes(schema.Triggers, schemaID, schemaPath)
		dbTableNodes, err := convertTablesToNodes(schema.Tables, schemaID, schemaPath, schema.Stats, indexesByTable, triggersByTable)
		if err != nil {
			return err
		}

		dbViewNodes, err := convertViewsToNodes(schema.Views, schemaID, schemaPath)
		if err != nil {
			return err
		}

		typeNodes := convertTypesToNodes(schema.Types, schemaID, schemaPath)
		funcNodes := convertFunctionsToNodes(schema.Functions, schemaID, schemaPath)
		settingsLabel := settingsSectionLabel(dbInstance.DBType)
		settingNodes := convertSettingsToNodes(schema.Settings, schemaID, schemaPath, settingsLabel)

		schemaExtraChildren := []*graph.DBInstanceItemNode{
			{
				ID:       fmt.Sprintf("%s:indexes", schemaID),
				Name:     "Indexes",
				Type:     "indexes",
				Path:     schemaPath + " / Indexes",
				Badges:   countBadge(dbIndexNodes),
				Children: dbIndexNodes,
			},
			{
				ID:       fmt.Sprintf("%s:triggers", schemaID),
				Name:     "Triggers",
				Type:     "triggers",
				Path:     schemaPath + " / Triggers",
				Badges:   countBadge(dbTriggerNodes),
				Children: dbTriggerNodes,
			},
			{
				ID:       fmt.Sprintf("%s:types", schemaID),
				Name:     "Types",
				Type:     "types",
				Path:     schemaPath + " / Types",
				Badges:   countBadge(typeNodes),
				Children: typeNodes,
			},
			{
				ID:       fmt.Sprintf("%s:functions", schemaID),
				Name:     "Functions",
				Type:     "functions",
				Path:     schemaPath + " / Functions",
				Badges:   countBadge(funcNodes),
				Children: funcNodes,
			},
			{
				ID:       fmt.Sprintf("%s:db_settings", schemaID),
				Name:     settingsLabel,
				Type:     "db_settings",
				Path:     schemaPath + " / " + settingsLabel,
				Badges:   countBadge(settingNodes),
				Children: settingNodes,
			},
		}

		schemaChildren := []*graph.DBInstanceItemNode{
			{
				ID:       fmt.Sprintf("%s:tables", schemaID),
				Name:     "Tables",
				Type:     "tables",
				Path:     schemaPath + " / Tables",
				Badges:   countBadge(dbTableNodes),
				Children: dbTableNodes,
			},
			{
				ID:       fmt.Sprintf("%s:views", schemaID),
				Name:     "Views",
				Type:     "views",
				Path:     schemaPath + " / Views",
				Badges:   countBadge(dbViewNodes),
				Children: dbViewNodes,
			},
		}
		schemaChildren = append(schemaChildren, schemaExtraChildren...)

		schemaNode := &graph.DBInstanceItemNode{
			ID:       schemaID,
			Name:     schema.Name,
			Type:     "schema",
			Path:     schemaPath,
			Children: schemaChildren,
		}
		schemaNodes = append(schemaNodes, schemaNode)
	}

	// Mutate the graph structure by updating the schema of the database instance
	_ = dbc.Graph.Mutate(dbc.ctx, generated.MutationCommit{
		Operation: "update",
		TableName: "db_instance",
		ObjectID:  dbInstance.ID,
		Payload: map[string]interface{}{
			"ID":       dbInstance.ID,
			"Children": schemaNodes,
		},
	})

	desktop.Emit("databaseAvailability", map[string]interface{}{
		"databases": []map[string]interface{}{
			{"id": dbInstance.ID},
		},
	})

	// Write schema.sql in the background so it doesn't block the UI.
	// The graph is already updated and the schema tree is visible at this point.
	noCache := queryParams.NoCache
	go func() {
		inst := engine.DBInstance{
			ID:        dbInstance.ID,
			DBType:    dbInstance.DBType,
			Proxified: dbInstance.Proxified,
		}

		dsn := ""
		if !dbInstance.Proxified {
			dsn = dbc.effectiveDSN(dbInstance)
		}

		schemaSQL := engineClient.DumpSchema(
			context.Background(),
			inst,
			dbInstance.WorkspaceID,
			dsn,
			metadata,
			noCache,
		)
		schemaFileURI := strings.TrimSuffix(dbInstance.URI, "/") + "/schema.sql"
		_ = dbc.FSProvider.Write(fs_provider.WriteParams{URI: schemaFileURI, Content: schemaSQL})
	}()

	return nil
}

// convertIndexesToNodes converts core.IndexInfo to graph nodes. It returns the
// flat list (used for the schema-level Indexes section) and the same nodes
// grouped by owning table name, so callers never have to recover the table from
// the node ID.
func convertIndexesToNodes(indexes []core.IndexInfo, dbInstanceID string, schemaPath string) ([]*graph.DBInstanceItemNode, map[string][]*graph.DBInstanceItemNode) {
	var indexNodes []*graph.DBInstanceItemNode
	byTable := make(map[string][]*graph.DBInstanceItemNode)

	for _, idx := range indexes {
		indexPath := schemaPath + " / " + idx.TableName + " / " + idx.Name

		// Convert index columns to graph nodes
		var columnNodes []*graph.DBInstanceItemNode
		for _, col := range idx.Columns {
			columnNodes = append(columnNodes, &graph.DBInstanceItemNode{
				ID:   fmt.Sprintf("%s:table:%s:index:%s:column:%s", dbInstanceID, idx.TableName, idx.Name, col.Name),
				Name: col.Name,
				Type: "index:column",
				Path: indexPath + " / " + col.Name,
				Metadata: map[string]any{
					"name":       col.Name,
					"position":   col.Position,
					"collation":  col.Collation,
					"descending": col.Descending,
				},
			})
		}

		indexNode := &graph.DBInstanceItemNode{
			ID:   fmt.Sprintf("%s:table:%s:index:%s", dbInstanceID, idx.TableName, idx.Name),
			Name: idx.Name,
			Type: "index",
			Path: indexPath,
			Metadata: map[string]any{
				"name":  idx.Name,
				"sql":   idx.DDL,
				"table": idx.TableName,
			},
			Children: columnNodes,
		}

		indexNodes = append(indexNodes, indexNode)
		byTable[idx.TableName] = append(byTable[idx.TableName], indexNode)
	}

	return indexNodes, byTable
}

// convertTriggersToNodes converts core.TriggerInfo to graph nodes. Like
// convertIndexesToNodes it returns both the flat list and a grouping by owning
// table name.
func convertTriggersToNodes(triggers []core.TriggerInfo, dbInstanceID string, schemaPath string) ([]*graph.DBInstanceItemNode, map[string][]*graph.DBInstanceItemNode) {
	var triggerNodes []*graph.DBInstanceItemNode
	byTable := make(map[string][]*graph.DBInstanceItemNode)

	for _, trigger := range triggers {
		triggerID := fmt.Sprintf("%s:table:%s:trigger:%s", dbInstanceID, trigger.TableName, trigger.Name)

		triggerNode := &graph.DBInstanceItemNode{
			ID:   triggerID,
			Name: trigger.Name,
			Type: "trigger",
			Path: schemaPath + " / " + trigger.TableName + " / " + trigger.Name,
			Metadata: map[string]any{
				"name":       trigger.Name,
				"table_name": trigger.TableName,
				"sql":        trigger.DDL,
			},
		}

		triggerNodes = append(triggerNodes, triggerNode)
		byTable[trigger.TableName] = append(byTable[trigger.TableName], triggerNode)
	}

	return triggerNodes, byTable
}

// columnMetadataFromCore builds the column metadata map matching the frontend Zod column schema:
// name, type, nullable, default, isPrimaryKey, isForeignKey, foreignKey (optional), extra (optional).
func columnMetadataFromCore(col core.Column) map[string]any {
	meta := map[string]any{
		"name":         col.Name,
		"type":         col.Type,
		"nullable":     col.Nullable,
		"default":      nil,
		"isPrimaryKey": col.IsPrimaryKey,
		"isForeignKey": col.IsForeignKey,
	}
	if col.Default != nil {
		meta["default"] = *col.Default
	}
	if col.IsForeignKey && col.ForeignKey != nil {
		meta["foreignKey"] = map[string]any{
			"schemaName": col.ForeignKey.SchemaName,
			"tableName":  col.ForeignKey.TableName,
			"columnName": col.ForeignKey.ColumnName,
		}
	}
	if len(col.Extra) > 0 {
		meta["extra"] = col.Extra
	}
	return meta
}

// convertTablesToNodes converts tables to DBInstanceItemNode with additional information
func convertTablesToNodes(
	tables []core.Table,
	dbID string,
	schemaPath string,
	stats core.TableStats,
	indexesByTable map[string][]*graph.DBInstanceItemNode,
	triggersByTable map[string][]*graph.DBInstanceItemNode,
) ([]*graph.DBInstanceItemNode, error) {
	var tableNodes []*graph.DBInstanceItemNode

	for _, table := range tables {
		objectID := fmt.Sprintf("%s:table:%s", dbID, table.Name)
		tablePath := schemaPath + " / " + table.Name
		stat := ""
		if stats != nil {
			stat = stats[table.Name]
		}

		// Look up this table's indexes and build per-column index metadata
		indexGroup := indexesByTable[table.Name]
		indexedColumns := make(map[string]bool)
		indexesByColumn := make(map[string][]*graph.DBInstanceItemNode)
		for _, idx := range indexGroup {
			for _, child := range idx.Children {
				if child.Type != "index:column" {
					continue
				}
				colName := child.Name
				indexedColumns[colName] = true
				indexesByColumn[colName] = append(indexesByColumn[colName], idx)
			}
		}

		// Convert columns to graph nodes with full metadata (matches frontend column schema)
		var columnNodes []*graph.DBInstanceItemNode
		for _, col := range table.Columns {
			meta := columnMetadataFromCore(col)

			// Index participation
			hasIdx := indexedColumns[col.Name]
			meta["hasIndex"] = hasIdx

			// Attach index nodes as children of the column in the graph
			children := indexesByColumn[col.Name]

			columnNodes = append(columnNodes, &graph.DBInstanceItemNode{
				ID:       fmt.Sprintf("%s:column:%s", objectID, col.Name),
				Name:     col.Name,
				Type:     fmt.Sprintf("column:%s", strings.ToLower(col.Type)),
				Path:     tablePath + " / " + col.Name,
				Metadata: meta,
				Children: children,
			})
		}

		columnsNode := &graph.DBInstanceItemNode{
			ID:       fmt.Sprintf("%s:columns", objectID),
			Name:     "Columns",
			Type:     "columns",
			Path:     tablePath + " / Columns",
			Badges:   countBadge(columnNodes),
			Children: columnNodes,
		}

		// Look up this table's triggers (indexGroup already computed above)
		triggersGroup := triggersByTable[table.Name]

		indexesNode := &graph.DBInstanceItemNode{
			ID:       fmt.Sprintf("%s:indexes", objectID),
			Name:     "Indexes",
			Type:     "indexes",
			Path:     tablePath + " / Indexes",
			Badges:   countBadge(indexGroup),
			Children: indexGroup,
		}

		triggersNode := &graph.DBInstanceItemNode{
			ID:       fmt.Sprintf("%s:triggers", objectID),
			Name:     "Triggers",
			Type:     "triggers",
			Path:     tablePath + " / Triggers",
			Badges:   countBadge(triggersGroup),
			Children: triggersGroup,
		}

		tableNode := &graph.DBInstanceItemNode{
			ID:   objectID,
			Name: table.Name,
			Type: "table",
			Path: tablePath,
			Metadata: map[string]any{
				"name": table.Name,
				"sql":  table.DDL,
				"stat": stat,
			},
			Children: []*graph.DBInstanceItemNode{
				columnsNode, indexesNode, triggersNode,
			},
		}

		tableNodes = append(tableNodes, tableNode)
	}

	return tableNodes, nil
}

// convertViewsToNodes converts views to DBInstanceItemNode with additional information
func convertViewsToNodes(
	views []core.Table,
	dbID string,
	schemaPath string,
) ([]*graph.DBInstanceItemNode, error) {
	var viewNodes []*graph.DBInstanceItemNode

	for _, view := range views {
		objectID := fmt.Sprintf("%s:view:%s", dbID, view.Name)
		viewPath := schemaPath + " / " + view.Name

		var columnNodes []*graph.DBInstanceItemNode
		for _, col := range view.Columns {
			columnNodes = append(columnNodes, &graph.DBInstanceItemNode{
				ID:       fmt.Sprintf("%s:column:%s", objectID, col.Name),
				Name:     col.Name,
				Type:     fmt.Sprintf("column:%s", strings.ToLower(col.Type)),
				Path:     viewPath + " / " + col.Name,
				Metadata: columnMetadataFromCore(col),
			})
		}

		columnsNode := &graph.DBInstanceItemNode{
			ID:       fmt.Sprintf("%s:columns", objectID),
			Name:     fmt.Sprintf("Columns (%d)", len(columnNodes)),
			Type:     "columns",
			Path:     viewPath + " / Columns",
			Children: columnNodes,
		}

		viewNode := &graph.DBInstanceItemNode{
			ID:   objectID,
			Name: view.Name,
			Type: "view",
			Path: viewPath,
			Metadata: map[string]any{
				"name": view.Name,
				"sql":  view.DDL,
			},
			Children: []*graph.DBInstanceItemNode{
				columnsNode,
			},
		}

		viewNodes = append(viewNodes, viewNode)
	}

	return viewNodes, nil
}

func convertTypesToNodes(types []core.Type, schemaID, schemaPath string) []*graph.DBInstanceItemNode {
	var nodes []*graph.DBInstanceItemNode
	for _, t := range types {
		disp := t.Display
		if disp == "" {
			disp = t.Name
		}
		nodes = append(nodes, &graph.DBInstanceItemNode{
			ID:   fmt.Sprintf("%s:type:%s", schemaID, t.Name),
			Name: disp,
			Type: "type",
			Path: schemaPath + " / Types / " + disp,
			Metadata: map[string]any{
				"name":        t.Name,
				"schema":      t.Schema,
				"kind":        t.Kind,
				"display":     disp,
				"description": t.Description,
				"enum_labels": t.EnumLabels,
			},
		})
	}
	return nodes
}

// settingsSectionLabel picks the dialect-native term for runtime parameters.
// Other sections use generic labels by convention; runtime parameters are
// the only one where the user-facing term genuinely differs across dialects.
func settingsSectionLabel(dbType string) string {
	switch dbType {
	case "mysql":
		return "Variables"
	case "sqlite":
		return "Pragmas"
	default:
		return "Settings"
	}
}

func convertSettingsToNodes(settings []core.Setting, schemaID, schemaPath, sectionLabel string) []*graph.DBInstanceItemNode {
	var nodes []*graph.DBInstanceItemNode
	for _, s := range settings {
		nodes = append(nodes, &graph.DBInstanceItemNode{
			ID:   fmt.Sprintf("%s:db_setting:%s", schemaID, s.Name),
			Name: s.Name,
			Type: "db_setting",
			Path: schemaPath + " / " + sectionLabel + " / " + s.Name,
			Metadata: map[string]any{
				"name":        s.Name,
				"value":       s.Value,
				"description": s.Description,
			},
		})
	}
	return nodes
}

func convertFunctionsToNodes(funcs []core.Function, schemaID, schemaPath string) []*graph.DBInstanceItemNode {
	var nodes []*graph.DBInstanceItemNode
	for _, f := range funcs {
		label := sqlFunctionDisplayLabel(f)
		var funcID string
		if f.OID != 0 {
			funcID = fmt.Sprintf("%s:function:%d", schemaID, f.OID)
		} else {
			funcID = fmt.Sprintf("%s:function:%s(%s)", schemaID, f.Name, f.Args)
		}
		nodes = append(nodes, &graph.DBInstanceItemNode{
			ID:   funcID,
			Name: label,
			Type: "function",
			Path: schemaPath + " / Functions / " + label,
			Metadata: map[string]any{
				"name":        f.Name,
				"schema":      f.Schema,
				"args":        f.Args,
				"result":      f.Result,
				"kind":        f.Kind,
				"description": f.Description,
				"oid":         f.OID,
			},
		})
	}
	return nodes
}

func countBadge[T any](items []T) []string {
	if len(items) == 0 {
		return []string{}
	}
	return []string{strconv.Itoa(len(items))}
}

func sqlFunctionDisplayLabel(f core.Function) string {
	if f.Args != "" {
		return f.Name + "(" + f.Args + ")"
	}
	return f.Name + "()"
}
