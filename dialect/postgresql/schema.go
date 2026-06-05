package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/lib/pq"
	core "github.com/selectDb/dialect/core"
	pgparser "github.com/selectDb/dialect/postgresql/parser"
)

// DefaultSchemaName implements core.SQLDialect.DefaultSchemaName.
func (d *Dialect) DefaultSchemaName() string { return "public" }

// GetCurrentSchema returns the current active schema for the PostgreSQL session.
func (d *Dialect) GetCurrentSchema(ctx context.Context, db *sql.DB) (string, error) {
	const query = `SELECT current_schema();`

	var currentSchema sql.NullString
	err := db.QueryRowContext(ctx, query).Scan(&currentSchema)
	if err != nil {
		return "", fmt.Errorf("failed to query current schema: %w", err)
	}

	if !currentSchema.Valid || currentSchema.String == "" {
		const searchPathQuery = `SHOW search_path;`
		var searchPath string
		if err := db.QueryRowContext(ctx, searchPathQuery).Scan(&searchPath); err != nil {
			return "public", nil
		}
		parts := strings.Split(searchPath, ",")
		if len(parts) > 0 {
			firstSchema := strings.TrimSpace(strings.Trim(parts[0], `"`))
			if firstSchema != "" {
				return firstSchema, nil
			}
		}
		return "public", nil
	}

	return currentSchema.String, nil
}

// GetSchemas returns all user schemas in the PostgreSQL database,
// excluding system schemas (pg_*, information_schema).
func (d *Dialect) GetSchemas(ctx context.Context, db *sql.DB) ([]string, error) {
	const query = `
		SELECT nspname
		FROM pg_catalog.pg_namespace
		WHERE nspname NOT LIKE 'pg_%'
		  AND nspname != 'information_schema'
		ORDER BY nspname ASC;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schemas []string
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}
		schemas = append(schemas, schemaName)
	}
	return schemas, rows.Err()
}

// GetTables returns all tables in the specified PostgreSQL schema.
// Uses 5 bulk queries total regardless of table count (was 5×N previously).
func (d *Dialect) GetTables(ctx context.Context, db *sql.DB, schema string) ([]core.Table, error) {
	// Phase 1: list table names
	const listQuery = `
		SELECT c.relname
		FROM pg_catalog.pg_class AS c
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relkind IN ('r', 'p')
		ORDER BY c.relname ASC;
	`
	rows, err := db.QueryContext(ctx, listQuery, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if len(tableNames) == 0 {
		return nil, nil
	}

	// Phase 2: bulk-fetch columns, PKs, FKs, DDL in parallel.
	var (
		wg                                  sync.WaitGroup
		allColumns                          map[string][]core.Column
		allPKs                              map[string][]string
		allFKs                              map[string]map[string]core.ForeignKeyRef
		allDDLs                             map[string]string
		errColumns, errPKs, errFKs, errDDLs error
	)
	wg.Add(4)
	go func() { defer wg.Done(); allColumns, errColumns = d.batchColumns(ctx, db, schema, tableNames) }()
	go func() { defer wg.Done(); allPKs, errPKs = d.batchPrimaryKeys(ctx, db, schema, tableNames) }()
	go func() { defer wg.Done(); allFKs, errFKs = d.batchForeignKeys(ctx, db, schema, tableNames) }()
	go func() { defer wg.Done(); allDDLs, errDDLs = d.batchTableDDL(ctx, db, schema, tableNames) }()
	wg.Wait()

	if errColumns != nil {
		return nil, fmt.Errorf("failed to batch columns: %w", errColumns)
	}
	if errPKs != nil {
		return nil, fmt.Errorf("failed to batch primary keys: %w", errPKs)
	}
	if errFKs != nil {
		return nil, fmt.Errorf("failed to batch foreign keys: %w", errFKs)
	}
	if errDDLs != nil {
		return nil, fmt.Errorf("failed to batch table DDL: %w", errDDLs)
	}

	// Phase 3: assemble
	tables := make([]core.Table, 0, len(tableNames))
	for _, name := range tableNames {
		cols := allColumns[name]
		pk := allPKs[name]
		fk := allFKs[name]
		core.EnrichColumnsWithConstraints(&cols, pk, fk)
		tables = append(tables, core.Table{
			Name:       name,
			Columns:    cols,
			PrimaryKey: pk,
			DDL:        allDDLs[name],
		})
	}
	return tables, nil
}

// batchColumns fetches columns for all given tables in one query.
// Returns map[tableName][]Column.
func (d *Dialect) batchColumns(ctx context.Context, db *sql.DB, schema string, tableNames []string) (map[string][]core.Column, error) {
	const query = `
		SELECT
			c.relname,
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod),
			a.attnotnull,
			pg_catalog.pg_get_expr(ad.adbin, ad.adrelid)
		FROM pg_catalog.pg_attribute AS a
		JOIN pg_catalog.pg_class AS c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_attrdef AS ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
		WHERE n.nspname = $1
		  AND c.relname = ANY($2::text[])
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY c.relname, a.attnum;
	`
	rows, err := db.QueryContext(ctx, query, schema, pq.Array(tableNames))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]core.Column, len(tableNames))
	for rows.Next() {
		var tableName, colName, dataType string
		var notNull bool
		var colDefault sql.NullString
		if err := rows.Scan(&tableName, &colName, &dataType, &notNull, &colDefault); err != nil {
			return nil, err
		}
		col := core.Column{Name: colName, Type: dataType, Nullable: !notNull}
		if colDefault.Valid && colDefault.String != "" {
			col.Default = &colDefault.String
		}
		result[tableName] = append(result[tableName], col)
	}
	return result, rows.Err()
}

// batchPrimaryKeys fetches primary key columns for all given tables in one query.
// Returns map[tableName][]columnName.
func (d *Dialect) batchPrimaryKeys(ctx context.Context, db *sql.DB, schema string, tableNames []string) (map[string][]string, error) {
	const query = `
		SELECT c.relname, a.attname
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
		WHERE con.contype = 'p'
		  AND n.nspname = $1
		  AND c.relname = ANY($2::text[])
		ORDER BY c.relname, array_position(con.conkey, a.attnum);
	`
	rows, err := db.QueryContext(ctx, query, schema, pq.Array(tableNames))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]string)
	for rows.Next() {
		var tableName, colName string
		if err := rows.Scan(&tableName, &colName); err != nil {
			return nil, err
		}
		result[tableName] = append(result[tableName], colName)
	}
	return result, rows.Err()
}

// batchForeignKeys fetches foreign key relationships for all given tables in one query.
// Returns map[tableName]map[localColumn]ForeignKeyRef.
func (d *Dialect) batchForeignKeys(ctx context.Context, db *sql.DB, schema string, tableNames []string) (map[string]map[string]core.ForeignKeyRef, error) {
	const query = `
		SELECT
			c.relname,
			a.attname,
			ref_n.nspname,
			ref_c.relname,
			ref_a.attname
		FROM pg_catalog.pg_constraint AS con
		JOIN pg_catalog.pg_class AS c ON c.oid = con.conrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord) ON true
		JOIN pg_catalog.pg_attribute AS a ON a.attrelid = c.oid AND a.attnum = k.attnum AND NOT a.attisdropped AND a.attnum > 0
		JOIN LATERAL unnest(con.confkey) WITH ORDINALITY AS r(attnum, ord) ON r.ord = k.ord
		JOIN pg_catalog.pg_class AS ref_c ON ref_c.oid = con.confrelid
		JOIN pg_catalog.pg_namespace AS ref_n ON ref_n.oid = ref_c.relnamespace
		JOIN pg_catalog.pg_attribute AS ref_a ON ref_a.attrelid = ref_c.oid AND ref_a.attnum = r.attnum AND NOT ref_a.attisdropped AND ref_a.attnum > 0
		WHERE con.contype = 'f'
		  AND n.nspname = $1
		  AND c.relname = ANY($2::text[]);
	`
	rows, err := db.QueryContext(ctx, query, schema, pq.Array(tableNames))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]map[string]core.ForeignKeyRef)
	for rows.Next() {
		var tableName, localCol, refSchema, refTable, refCol string
		if err := rows.Scan(&tableName, &localCol, &refSchema, &refTable, &refCol); err != nil {
			return nil, err
		}
		if result[tableName] == nil {
			result[tableName] = make(map[string]core.ForeignKeyRef)
		}
		result[tableName][localCol] = core.ForeignKeyRef{
			SchemaName: refSchema,
			TableName:  refTable,
			ColumnName: refCol,
		}
	}
	return result, rows.Err()
}

// batchTableDDL builds full CREATE TABLE DDL for all given tables in two queries
// (columns aggregation + constraints aggregation joined via CTE), replacing the
// previous per-table two-query approach.
// Returns map[tableName]ddlString.
func (d *Dialect) batchTableDDL(ctx context.Context, db *sql.DB, schema string, tableNames []string) (map[string]string, error) {
	const query = `
		WITH cols AS (
			SELECT
				c.relname AS table_name,
				c.oid     AS table_oid,
				'CREATE TABLE ' || quote_ident(n.nspname) || '.' || quote_ident(c.relname) || E' (\n' ||
				string_agg(
					'    ' || quote_ident(a.attname) || ' ' ||
					pg_catalog.format_type(a.atttypid, a.atttypmod) ||
					CASE WHEN a.attnotnull THEN ' NOT NULL' ELSE '' END ||
					CASE WHEN ad.adbin IS NOT NULL
						THEN ' DEFAULT ' || pg_catalog.pg_get_expr(ad.adbin, ad.adrelid)
						ELSE ''
					END,
					E',\n' ORDER BY a.attnum
				) AS columns_ddl
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid
			LEFT JOIN pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
			WHERE c.relkind IN ('r', 'p')
			  AND a.attnum > 0
			  AND NOT a.attisdropped
			  AND n.nspname = $1
			  AND c.relname = ANY($2::text[])
			GROUP BY n.nspname, c.relname, c.oid
		),
		cons AS (
			SELECT
				con.conrelid AS table_oid,
				string_agg(
					'    CONSTRAINT ' || quote_ident(con.conname) || ' ' ||
					pg_get_constraintdef(con.oid, true),
					E',\n'
					ORDER BY
						CASE con.contype
							WHEN 'p' THEN 1
							WHEN 'u' THEN 2
							WHEN 'c' THEN 3
							WHEN 'f' THEN 4
							ELSE 5
						END,
						con.conname
				) AS constraints_ddl
			FROM pg_constraint con
			WHERE con.conrelid = ANY (SELECT table_oid FROM cols)
			GROUP BY con.conrelid
		)
		SELECT
			cols.table_name,
			cols.columns_ddl ||
			CASE WHEN cons.constraints_ddl IS NOT NULL
				THEN E',\n\n' || cons.constraints_ddl
				ELSE ''
			END || E'\n);'
		FROM cols
		LEFT JOIN cons ON cons.table_oid = cols.table_oid;
	`

	rows, err := db.QueryContext(ctx, query, schema, pq.Array(tableNames))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]string, len(tableNames))
	for rows.Next() {
		var tableName, rawDDL string
		if err := rows.Scan(&tableName, &rawDDL); err != nil {
			return nil, err
		}
		result[tableName] = core.FormatTableDDL(rawDDL)
	}
	return result, rows.Err()
}

// GetViews returns all views in the specified PostgreSQL schema.
// Columns are bulk-fetched in one query.
func (d *Dialect) GetViews(ctx context.Context, db *sql.DB, schema string) ([]core.Table, error) {
	const listQuery = `
		SELECT c.relname, pg_catalog.pg_get_viewdef(c.oid, true)
		FROM pg_catalog.pg_class AS c
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind = 'v'
		ORDER BY c.relname ASC;
	`
	rows, err := db.QueryContext(ctx, listQuery, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query views: %w", err)
	}

	type viewRow struct {
		name string
		def  sql.NullString
	}
	var viewRows []viewRow
	var viewNames []string
	for rows.Next() {
		var r viewRow
		if err := rows.Scan(&r.name, &r.def); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan view row: %w", err)
		}
		viewRows = append(viewRows, r)
		viewNames = append(viewNames, r.name)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if len(viewNames) == 0 {
		return nil, nil
	}

	allColumns, err := d.batchColumns(ctx, db, schema, viewNames)
	if err != nil {
		return nil, fmt.Errorf("failed to batch view columns: %w", err)
	}

	views := make([]core.Table, 0, len(viewRows))
	for _, r := range viewRows {
		definition := strings.TrimSpace(getString(r.def))
		ddl := ""
		if definition != "" {
			ddl = fmt.Sprintf("CREATE VIEW %s AS\n%s", r.name, definition)
		}
		views = append(views, core.Table{
			Name:    r.name,
			Columns: allColumns[r.name],
			DDL:     ddl,
		})
	}
	return views, nil
}

// GetIndexes returns index metadata for the specified PostgreSQL schema.
// Index columns are bulk-fetched in one query instead of one per index.
func (d *Dialect) GetIndexes(ctx context.Context, db *sql.DB, schema string) ([]core.IndexInfo, error) {
	const listQuery = `
		SELECT i.relname, t.relname, pg_catalog.pg_get_indexdef(i.oid), i.oid::bigint
		FROM pg_catalog.pg_class AS i
		JOIN pg_catalog.pg_index AS ix ON ix.indexrelid = i.oid
		JOIN pg_catalog.pg_class AS t ON t.oid = ix.indrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = t.relnamespace
		WHERE n.nspname = $1
		ORDER BY t.relname, i.relname;
	`
	rows, err := db.QueryContext(ctx, listQuery, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}

	type indexRow struct {
		name, table string
		ddl         sql.NullString
		oid         int64
	}
	var indexRows []indexRow
	var oids []int64
	for rows.Next() {
		var r indexRow
		if err := rows.Scan(&r.name, &r.table, &r.ddl, &r.oid); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}
		indexRows = append(indexRows, r)
		oids = append(oids, r.oid)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if len(oids) == 0 {
		return nil, nil
	}

	// Bulk-fetch all index columns in one query
	allCols, err := d.batchIndexColumns(ctx, db, oids)
	if err != nil {
		return nil, fmt.Errorf("failed to batch index columns: %w", err)
	}

	indexes := make([]core.IndexInfo, 0, len(indexRows))
	for _, r := range indexRows {
		indexes = append(indexes, core.IndexInfo{
			Name:      r.name,
			TableName: r.table,
			DDL:       getString(r.ddl),
			Columns:   allCols[r.oid],
		})
	}
	return indexes, nil
}

// batchIndexColumns fetches column info for all given index OIDs in one query.
// Returns map[indexOID][]IndexColumnInfo.
func (d *Dialect) batchIndexColumns(ctx context.Context, db *sql.DB, oids []int64) (map[int64][]core.IndexColumnInfo, error) {
	const query = `
		SELECT
			ix.indexrelid::bigint,
			COALESCE(a.attname, ''),
			cols.ordinality,
			COALESCE(coll.collname, ''),
			(ix.indoption[cols.ordinality - 1] & 1) = 1
		FROM pg_catalog.pg_index AS ix
		JOIN pg_catalog.pg_class AS t ON t.oid = ix.indrelid
		JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS cols(attnum, ordinality) ON true
		LEFT JOIN pg_catalog.pg_attribute AS a ON a.attrelid = t.oid AND a.attnum = cols.attnum
		LEFT JOIN pg_catalog.pg_collation AS coll ON coll.oid = ix.indcollation[cols.ordinality - 1]
		WHERE ix.indexrelid = ANY($1::bigint[])
		  AND cols.attnum <> 0
		ORDER BY ix.indexrelid, cols.ordinality;
	`
	rows, err := db.QueryContext(ctx, query, pq.Array(oids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]core.IndexColumnInfo)
	for rows.Next() {
		var oid int64
		var colName, collation string
		var position int
		var descending bool
		if err := rows.Scan(&oid, &colName, &position, &collation, &descending); err != nil {
			return nil, err
		}
		if colName == "" {
			continue // expression columns skipped
		}
		result[oid] = append(result[oid], core.IndexColumnInfo{
			Name:       colName,
			Position:   position,
			Collation:  collation,
			Descending: descending,
		})
	}
	return result, rows.Err()
}

// GetTriggers returns trigger metadata for the specified PostgreSQL schema.
func (d *Dialect) GetTriggers(ctx context.Context, db *sql.DB, schema string) ([]core.TriggerInfo, error) {
	const query = `
		SELECT tg.tgname, tbl.relname, pg_catalog.pg_get_triggerdef(tg.oid, true)
		FROM pg_catalog.pg_trigger AS tg
		JOIN pg_catalog.pg_class AS tbl ON tbl.oid = tg.tgrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = tbl.relnamespace
		WHERE NOT tg.tgisinternal AND n.nspname = $1
		ORDER BY tg.tgname ASC;
	`
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query triggers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var triggers []core.TriggerInfo
	for rows.Next() {
		var name, table string
		var ddl sql.NullString
		if err := rows.Scan(&name, &table, &ddl); err != nil {
			return nil, fmt.Errorf("failed to scan trigger row: %w", err)
		}
		triggers = append(triggers, core.TriggerInfo{Name: name, TableName: table, DDL: getString(ddl)})
	}
	return triggers, rows.Err()
}

// GetStats returns basic table and index statistics for the specified schema.
func (d *Dialect) GetStats(ctx context.Context, db *sql.DB, schema string) (core.TableStats, error) {
	stats := make(core.TableStats)

	tableRows, err := db.QueryContext(ctx, `
		SELECT relname, n_live_tup::text
		FROM pg_catalog.pg_stat_user_tables
		WHERE schemaname = $1;
	`, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query table statistics: %w", err)
	}
	defer func() { _ = tableRows.Close() }()
	for tableRows.Next() {
		var name, val string
		if err := tableRows.Scan(&name, &val); err != nil {
			return nil, err
		}
		stats[name] = val
	}
	if err := tableRows.Err(); err != nil {
		return nil, err
	}

	indexRows, err := db.QueryContext(ctx, `
		SELECT indexrelname, COALESCE(idx_scan, 0)::text
		FROM pg_catalog.pg_stat_user_indexes
		WHERE schemaname = $1;
	`, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query index statistics: %w", err)
	}
	defer func() { _ = indexRows.Close() }()
	for indexRows.Next() {
		var name, val string
		if err := indexRows.Scan(&name, &val); err != nil {
			return nil, err
		}
		stats[name] = val
	}
	return stats, indexRows.Err()
}

// GetTypes returns SQL types for the given user schema.
func (d *Dialect) GetTypes(ctx context.Context, db *sql.DB, schema string) ([]core.Type, error) {
	return loadPGTypesForSchema(ctx, db, schema)
}

// GetFunctions returns callable routines for the given user schema.
func (d *Dialect) GetFunctions(ctx context.Context, db *sql.DB, schema string) ([]core.Function, error) {
	return loadUserSchemaFunctions(ctx, db, []string{schema})
}

// GetCatalogSchema returns pg_catalog as a fully-populated Schema.
// Called lazily for lint/completion, not during regular schema load.
func (d *Dialect) GetCatalogSchema(ctx context.Context, db *sql.DB) (*core.Schema, error) {
	tables, err := d.GetTables(ctx, db, "pg_catalog")
	if err != nil {
		return nil, err
	}
	views, err := d.GetViews(ctx, db, "pg_catalog")
	if err != nil {
		return nil, err
	}
	types, err := loadPGTypesForSchema(ctx, db, "pg_catalog")
	if err != nil {
		return nil, err
	}
	funcs, err := loadPGCatalogBuiltinsMatchingList(ctx, db, nonPgBuiltinNames())
	if err != nil {
		return nil, err
	}
	return &core.Schema{
		Name:      "pg_catalog",
		Tables:    tables,
		Views:     views,
		Types:     types,
		Functions: funcs,
	}, nil
}

// GetSettings returns PostgreSQL runtime parameters (GUCs) with current
// value and short description from pg_settings.
func (d *Dialect) GetSettings(ctx context.Context, db *sql.DB) ([]core.Setting, error) {
	const query = `SELECT name, setting, COALESCE(short_desc, '') FROM pg_settings ORDER BY name;`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pg_settings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.Setting
	for rows.Next() {
		var name, value, desc string
		if err := rows.Scan(&name, &value, &desc); err != nil {
			return nil, fmt.Errorf("failed to scan pg_settings row: %w", err)
		}
		out = append(out, core.Setting{Name: name, Value: value, Description: desc})
	}
	return out, rows.Err()
}

// Helper
func getString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func loadPGTypesForSchema(ctx context.Context, db *sql.DB, schema string) ([]core.Type, error) {
	const q = `
SELECT n.nspname, t.typname, t.typtype::text,
       COALESCE(pg_catalog.format_type(t.oid, NULL), t.typname::text),
       obj_description(t.oid, 'pg_type'),
       (SELECT string_agg(e.enumlabel, E'\x01' ORDER BY e.enumsortorder)
        FROM pg_catalog.pg_enum e WHERE e.enumtypid = t.oid)
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
WHERE t.typisdefined
  AND n.nspname = $1
  AND t.typcategory NOT IN ('p', 'x')
  AND t.typelem = 0
  AND t.typname NOT LIKE 'pg_%'
ORDER BY t.typname
`
	rows, err := db.QueryContext(ctx, q, schema)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []core.Type
	for rows.Next() {
		var schemaName, name, kind, display string
		var description, enumRaw sql.NullString
		if err := rows.Scan(&schemaName, &name, &kind, &display, &description, &enumRaw); err != nil {
			return nil, err
		}
		ct := core.Type{
			Schema:      schemaName,
			Name:        name,
			Kind:        kind,
			Display:     display,
			Description: strings.TrimSpace(getString(description)),
		}
		if enumRaw.Valid && enumRaw.String != "" {
			ct.EnumLabels = strings.Split(enumRaw.String, "\x01")
		}
		out = append(out, ct)
	}
	return out, rows.Err()
}

func loadUserSchemaFunctions(ctx context.Context, db *sql.DB, userSchemas []string) ([]core.Function, error) {
	const q = `
SELECT p.oid::bigint, n.nspname, p.proname,
       pg_catalog.pg_get_function_identity_arguments(p.oid),
       pg_catalog.pg_get_function_result(p.oid),
       p.prokind::text,
       obj_description(p.oid, 'pg_proc')
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON p.pronamespace = n.oid
WHERE n.nspname = ANY($1::text[])
  AND n.nspname NOT IN ('pg_toast', 'information_schema', 'pg_catalog')
ORDER BY n.nspname, p.proname, p.oid
`
	rows, err := db.QueryContext(ctx, q, pq.Array(userSchemas))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []core.Function
	for rows.Next() {
		var oid int64
		var schema, name, args, result, kind string
		var description sql.NullString
		if err := rows.Scan(&oid, &schema, &name, &args, &result, &kind, &description); err != nil {
			return nil, err
		}
		out = append(out, core.Function{
			OID: oid, Schema: schema, Name: name, Args: args,
			Result: result, Kind: kind,
			Description: strings.TrimSpace(getString(description)),
		})
	}
	return out, rows.Err()
}

func nonPgBuiltinNames() []string {
	seen := make(map[string]struct{})
	var names []string
	for _, f := range pgparser.GetBuiltinFunctions() {
		if strings.HasPrefix(f, "pg_") {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		names = append(names, f)
	}
	return names
}

func loadPGCatalogBuiltinsMatchingList(ctx context.Context, db *sql.DB, names []string) ([]core.Function, error) {
	if len(names) == 0 {
		return nil, nil
	}
	const q = `
SELECT p.oid::bigint, n.nspname, p.proname,
       pg_catalog.pg_get_function_identity_arguments(p.oid),
       pg_catalog.pg_get_function_result(p.oid),
       p.prokind::text,
       obj_description(p.oid, 'pg_proc')
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON p.pronamespace = n.oid
WHERE n.nspname = 'pg_catalog'
  AND p.proname = ANY($1::text[])
ORDER BY p.proname, p.oid
`
	rows, err := db.QueryContext(ctx, q, pq.Array(names))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []core.Function
	for rows.Next() {
		var oid int64
		var schema, name, args, result, kind string
		var description sql.NullString
		if err := rows.Scan(&oid, &schema, &name, &args, &result, &kind, &description); err != nil {
			return nil, err
		}
		out = append(out, core.Function{
			OID: oid, Schema: schema, Name: name, Args: args,
			Result: result, Kind: kind,
			Description: strings.TrimSpace(getString(description)),
		})
	}
	return out, rows.Err()
}
