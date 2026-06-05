package engine

import (
	"strings"

	"github.com/selectDb/dialect/core"
)

// DumpSchema returns authoritative schema SQL for the given DSN. It tries
// the dialect's native CLI tool first (pg_dump, mysqldump); if that is
// unavailable it falls back to reconstructing DDL from metadata with a
// warning comment prepended. On the proxy the caller must pass a
// ResolveDumpDSN-pinned DSN: the CLI tools re-resolve and can't use the Go guard.
func DumpSchema(dialect core.SQLDialect, dsn string, metadata *core.Metadata) string {
	if sql, ok := dialect.DumpSchema(dsn); ok {
		return sql
	}
	return dialect.DumpSchemaWarning() + buildSchemaSQLFromMetadata(metadata)
}

// buildSchemaSQLFromMetadata reconstructs DDL from catalog metadata.
// Indexes and triggers appear immediately after their parent table.
func buildSchemaSQLFromMetadata(metadata *core.Metadata) string {
	var parts []string
	multiSchema := len(metadata.Schemas) > 1

	for _, schema := range metadata.Schemas {
		var section []string
		if multiSchema {
			section = append(section, strings.TrimRight("-- Schema: "+schema.Name, " \t\n\r"))
		}

		indexesByTable := make(map[string][]core.IndexInfo)
		for _, index := range schema.Indexes {
			indexesByTable[index.TableName] = append(indexesByTable[index.TableName], index)
		}
		triggersByTable := make(map[string][]core.TriggerInfo)
		for _, trigger := range schema.Triggers {
			triggersByTable[trigger.TableName] = append(triggersByTable[trigger.TableName], trigger)
		}

		emitDDL := func(ddl string) {
			ddl = strings.TrimRight(ddl, " \t\n\r")
			if ddl != "" {
				section = append(section, ddl)
			}
		}

		for _, table := range schema.Tables {
			emitDDL(table.DDL)
			for _, index := range indexesByTable[table.Name] {
				emitDDL(index.DDL)
			}
			for _, trigger := range triggersByTable[table.Name] {
				emitDDL(trigger.DDL)
			}
		}
		for _, view := range schema.Views {
			emitDDL(view.DDL)
			for _, index := range indexesByTable[view.Name] {
				emitDDL(index.DDL)
			}
		}
		for _, materializedView := range schema.MaterializedViews {
			emitDDL(materializedView.DDL)
			for _, index := range indexesByTable[materializedView.Name] {
				emitDDL(index.DDL)
			}
		}

		seen := make(map[string]bool)
		for _, table := range schema.Tables {
			seen[table.Name] = true
		}
		for _, view := range schema.Views {
			seen[view.Name] = true
		}
		for _, materializedView := range schema.MaterializedViews {
			seen[materializedView.Name] = true
		}
		for name, indexes := range indexesByTable {
			if seen[name] {
				continue
			}
			for _, index := range indexes {
				emitDDL(index.DDL)
			}
		}
		for name, triggers := range triggersByTable {
			if seen[name] {
				continue
			}
			for _, trigger := range triggers {
				emitDDL(trigger.DDL)
			}
		}

		if len(section) > 0 {
			parts = append(parts, strings.Join(section, "\n\n"))
		}
	}

	return strings.Join(parts, "\n\n")
}
