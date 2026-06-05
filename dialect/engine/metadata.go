package engine

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"

	"github.com/selectDb/dialect/core"
)

// hashWorkspaceDSN hashes (workspaceID, dsn) so DSNs never live in any
// cache map. Shared by metadata and connection caches. FNV-1a 64-bit:
// stdlib, alloc-free, ns-scale. Non-crypto;
func hashWorkspaceDSN(workspaceID, dsn string) string {
	h := fnv.New64a()
	h.Write([]byte(workspaceID))
	h.Write([]byte{0})
	h.Write([]byte(dsn))
	return strconv.FormatUint(h.Sum64(), 16)
}

// FetchMetadata loads full schema info (schemas/tables/views/indexes/
// triggers/stats/types/functions) into a *core.Metadata. Uncached;
// most callers want GetOrFetchMetadata. ctx bounds every query.
func FetchMetadata(ctx context.Context, db *sql.DB, dialect core.SQLDialect, dbName string) (*core.Metadata, error) {
	if db == nil {
		return nil, fmt.Errorf("FetchMetadata: db is nil")
	}
	if dialect == nil {
		return nil, fmt.Errorf("FetchMetadata: dialect is nil")
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

	type schemaResult struct {
		schema core.Schema
		err    error
	}
	results := make([]schemaResult, len(schemaNames))

	var wg sync.WaitGroup
	for i, schemaName := range schemaNames {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()

			var (
				swg                 sync.WaitGroup
				tables              []core.Table
				views               []core.Table
				indexes             []core.IndexInfo
				triggers            []core.TriggerInfo
				stats               core.TableStats
				types               []core.Type
				functions           []core.Function
				errTables, errViews error
			)
			swg.Add(7)
			go func() { defer swg.Done(); tables, errTables = dialect.GetTables(ctx, db, name) }()
			go func() { defer swg.Done(); views, errViews = dialect.GetViews(ctx, db, name) }()
			go func() { defer swg.Done(); indexes, _ = dialect.GetIndexes(ctx, db, name) }()
			go func() { defer swg.Done(); triggers, _ = dialect.GetTriggers(ctx, db, name) }()
			go func() { defer swg.Done(); stats, _ = dialect.GetStats(ctx, db, name) }()
			go func() { defer swg.Done(); types, _ = dialect.GetTypes(ctx, db, name) }()
			go func() { defer swg.Done(); functions, _ = dialect.GetFunctions(ctx, db, name) }()
			swg.Wait()

			if errTables != nil {
				results[idx].err = fmt.Errorf("failed to get tables for schema %s: %w", name, errTables)
				return
			}
			if errViews != nil {
				results[idx].err = fmt.Errorf("failed to get views for schema %s: %w", name, errViews)
				return
			}
			results[idx].schema = core.Schema{
				Name:              name,
				Tables:            tables,
				ForeignTables:     []core.Table{},
				Views:             views,
				MaterializedViews: []core.Table{},
				Indexes:           indexes,
				Triggers:          triggers,
				Stats:             stats,
				Types:             types,
				Functions:         functions,
			}
		}(i, schemaName)
	}
	wg.Wait()

	var schemas []core.Schema
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		schemas = append(schemas, r.schema)
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
