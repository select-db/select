// Package schema reads what a database holds (schemas, tables, columns,
// indexes) and dumps its DDL, both cached per workspace.
package schema

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/selectDb/dialect/core"
)

// Fetch loads the whole schema through the dialect's ReadSchema. Uncached;
// most callers want GetOrFetch. ctx bounds every query.
func Fetch(ctx context.Context, db *sql.DB, dialect core.SQLDialect, dbName string) (*core.Metadata, error) {
	if db == nil {
		return nil, fmt.Errorf("Fetch: db is nil")
	}
	if dialect == nil {
		return nil, fmt.Errorf("Fetch: dialect is nil")
	}

	meta, err := dialect.ReadSchema(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}
	meta.DefaultDB = dbName
	meta.DefaultSchema = meta.CurrentSchema
	core.EnrichEnumValues(meta)
	return meta, nil
}
