// Package schema reads what a database holds (schemas, tables, columns,
// indexes) and dumps its DDL, both cached per workspace.
package schema

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/selectDb/dialect/core"
)

// Fetch loads full schema info (schemas/tables/views/indexes/
// triggers/stats/types/functions) into a *core.Metadata. Uncached;
// most callers want GetOrFetch. ctx bounds every query.
//
// The queries run one after another, so a load holds one pooled connection:
// a fan-out opens as many as it runs at once, which a remote may refuse.
func Fetch(ctx context.Context, db *sql.DB, dialect core.SQLDialect, dbName string) (*core.Metadata, error) {
	if db == nil {
		return nil, fmt.Errorf("Fetch: db is nil")
	}
	if dialect == nil {
		return nil, fmt.Errorf("Fetch: dialect is nil")
	}

	schemaNames, err := dialect.GetSchemas(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get schemas: %w", err)
	}

	currentSchema, _ := dialect.GetCurrentSchema(ctx, db)
	defaultSchema := currentSchema
	if defaultSchema == "" {
		defaultSchema = dialect.DefaultSchemaName()
		currentSchema = defaultSchema
	}

	tables, err := dialect.GetTables(ctx, db, schemaNames)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	views, err := dialect.GetViews(ctx, db, schemaNames)
	if err != nil {
		return nil, fmt.Errorf("failed to get views: %w", err)
	}
	// The rest is detail: a dialect or server without it still has a schema.
	indexes, _ := dialect.GetIndexes(ctx, db, schemaNames)
	triggers, _ := dialect.GetTriggers(ctx, db, schemaNames)
	stats, _ := dialect.GetStats(ctx, db, schemaNames)
	types, _ := dialect.GetTypes(ctx, db, schemaNames)
	functions, _ := dialect.GetFunctions(ctx, db, schemaNames)

	var schemas []core.Schema
	for _, name := range schemaNames {
		schemas = append(schemas, core.Schema{
			Name:              name,
			Tables:            tables[name],
			ForeignTables:     []core.Table{},
			Views:             views[name],
			MaterializedViews: []core.Table{},
			Indexes:           indexes[name],
			Triggers:          triggers[name],
			Stats:             stats[name],
			Types:             types[name],
			Functions:         functions[name],
		})
	}

	if catSchema, err := dialect.GetCatalogSchema(ctx, db); err == nil && catSchema != nil {
		if settings, sErr := dialect.GetSettings(ctx, db); sErr == nil {
			catSchema.Settings = settings
		}
		schemas = append(schemas, *catSchema)
	}

	meta := &core.Metadata{
		DefaultDB:     dbName,
		DefaultSchema: defaultSchema,
		CurrentSchema: currentSchema,
		Schemas:       schemas,
	}
	core.EnrichEnumValues(meta)
	return meta, nil
}
