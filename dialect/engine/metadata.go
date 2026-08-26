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

// defaultMetadataConcurrency bounds how many introspection queries FetchMetadata
// runs at once when the caller doesn't specify. Metadata fetches fan out one
// query per schema × object type; left unbounded they can open dozens of pooled
// connections at once and trip the remote's max_connections ("too many clients").
const defaultMetadataConcurrency = 8

// FetchMetadata loads full schema info (schemas/tables/views/indexes/
// triggers/stats/types/functions) into a *core.Metadata. Uncached;
// most callers want GetOrFetchMetadata. ctx bounds every query.
//
// maxConcurrency optionally caps how many introspection queries run at once
// across all schemas. Unset or <=0 uses defaultMetadataConcurrency. The desktop
// app passes 1 so the whole load serialises onto a single pooled connection,
// staying well under the remote's connection limit; user queries then reuse the
// same cached pool.
func FetchMetadata(ctx context.Context, db *sql.DB, dialect core.SQLDialect, dbName string, maxConcurrency ...int) (*core.Metadata, error) {
	if db == nil {
		return nil, fmt.Errorf("FetchMetadata: db is nil")
	}
	if dialect == nil {
		return nil, fmt.Errorf("FetchMetadata: dialect is nil")
	}

	// A shared buffered channel bounds concurrent DB calls across every schema.
	limit := defaultMetadataConcurrency
	if len(maxConcurrency) > 0 && maxConcurrency[0] > 0 {
		limit = maxConcurrency[0]
	}
	sem := make(chan struct{}, limit)
	acquire := func() { sem <- struct{}{} }
	release := func() { <-sem }

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
			// run executes fn in its own goroutine, but only holds a DB
			// connection while inside the semaphore. limit=1 => fully serial.
			run := func(fn func()) {
				go func() {
					defer swg.Done()
					acquire()
					defer release()
					fn()
				}()
			}
			swg.Add(7)
			run(func() { tables, errTables = dialect.GetTables(ctx, db, name) })
			run(func() { views, errViews = dialect.GetViews(ctx, db, name) })
			run(func() { indexes, _ = dialect.GetIndexes(ctx, db, name) })
			run(func() { triggers, _ = dialect.GetTriggers(ctx, db, name) })
			run(func() { stats, _ = dialect.GetStats(ctx, db, name) })
			run(func() { types, _ = dialect.GetTypes(ctx, db, name) })
			run(func() { functions, _ = dialect.GetFunctions(ctx, db, name) })
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
