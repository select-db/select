package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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

// relKey identifies a relation across the schemas of one batch query.
type relKey struct{ schema, name string }

// GetTables returns the tables of each schema in five queries, whatever the
// number of schemas or tables.
func (d *Dialect) GetTables(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.Table, error) {
	const listQuery = `
		SELECT n.nspname, c.relname, pg_catalog.obj_description(c.oid, 'pg_class')
		FROM pg_catalog.pg_class AS c
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		WHERE n.nspname = ANY($1::text[])
		  AND c.relkind IN ('r', 'p')
		ORDER BY n.nspname, c.relname ASC;
	`
	rows, err := db.QueryContext(ctx, listQuery, pq.Array(schemas))
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	var keys []relKey
	tableComments := make(map[relKey]string)
	for rows.Next() {
		var k relKey
		var comment sql.NullString
		if err := rows.Scan(&k.schema, &k.name, &comment); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		keys = append(keys, k)
		tableComments[k] = comment.String
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return map[string][]core.Table{}, nil
	}

	allColumns, err := d.batchColumns(ctx, db, schemas, []string{"r", "p"})
	if err != nil {
		return nil, fmt.Errorf("failed to batch columns: %w", err)
	}
	allPKs, err := d.batchPrimaryKeys(ctx, db, schemas)
	if err != nil {
		return nil, fmt.Errorf("failed to batch primary keys: %w", err)
	}
	allFKs, err := d.batchForeignKeys(ctx, db, schemas)
	if err != nil {
		return nil, fmt.Errorf("failed to batch foreign keys: %w", err)
	}
	allDDLs, err := d.batchTableDDL(ctx, db, schemas)
	if err != nil {
		return nil, fmt.Errorf("failed to batch table DDL: %w", err)
	}

	tables := make(map[string][]core.Table)
	for _, k := range keys {
		cols := allColumns[k]
		pk := allPKs[k]
		core.EnrichColumnsWithConstraints(&cols, pk, allFKs[k])
		tables[k.schema] = append(tables[k.schema], core.Table{
			Name:        k.name,
			Columns:     cols,
			PrimaryKey:  pk,
			DDL:         allDDLs[k],
			Description: tableComments[k],
		})
	}
	return tables, nil
}

// batchColumns fetches the columns of every relation of the given relkinds in
// the given schemas in one query.
func (d *Dialect) batchColumns(ctx context.Context, db *sql.DB, schemas, relkinds []string) (map[relKey][]core.Column, error) {
	const query = `
		SELECT
			n.nspname,
			c.relname,
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod),
			a.attnotnull,
			pg_catalog.pg_get_expr(ad.adbin, ad.adrelid),
			pg_catalog.col_description(c.oid, a.attnum)
		FROM pg_catalog.pg_attribute AS a
		JOIN pg_catalog.pg_class AS c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_attrdef AS ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
		WHERE n.nspname = ANY($1::text[])
		  AND c.relkind::text = ANY($2::text[])
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY n.nspname, c.relname, a.attnum;
	`
	rows, err := db.QueryContext(ctx, query, pq.Array(schemas), pq.Array(relkinds))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[relKey][]core.Column)
	for rows.Next() {
		var k relKey
		var colName, dataType string
		var notNull bool
		var colDefault, colComment sql.NullString
		if err := rows.Scan(&k.schema, &k.name, &colName, &dataType, &notNull, &colDefault, &colComment); err != nil {
			return nil, err
		}
		col := core.Column{Name: colName, Type: dataType, Nullable: !notNull, Description: colComment.String}
		if colDefault.Valid && colDefault.String != "" {
			col.Default = &colDefault.String
		}
		result[k] = append(result[k], col)
	}
	return result, rows.Err()
}

// batchPrimaryKeys fetches the primary key columns of every table in the
// given schemas in one query.
func (d *Dialect) batchPrimaryKeys(ctx context.Context, db *sql.DB, schemas []string) (map[relKey][]string, error) {
	const query = `
		SELECT n.nspname, c.relname, a.attname
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
		WHERE con.contype = 'p'
		  AND n.nspname = ANY($1::text[])
		ORDER BY n.nspname, c.relname, array_position(con.conkey, a.attnum);
	`
	rows, err := db.QueryContext(ctx, query, pq.Array(schemas))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[relKey][]string)
	for rows.Next() {
		var k relKey
		var colName string
		if err := rows.Scan(&k.schema, &k.name, &colName); err != nil {
			return nil, err
		}
		result[k] = append(result[k], colName)
	}
	return result, rows.Err()
}

// batchForeignKeys fetches the foreign keys of every table in the given
// schemas in one query, as local column to referenced column.
func (d *Dialect) batchForeignKeys(ctx context.Context, db *sql.DB, schemas []string) (map[relKey]map[string]core.ForeignKeyRef, error) {
	const query = `
		SELECT
			n.nspname,
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
		  AND n.nspname = ANY($1::text[]);
	`
	rows, err := db.QueryContext(ctx, query, pq.Array(schemas))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[relKey]map[string]core.ForeignKeyRef)
	for rows.Next() {
		var k relKey
		var localCol, refSchema, refTable, refCol string
		if err := rows.Scan(&k.schema, &k.name, &localCol, &refSchema, &refTable, &refCol); err != nil {
			return nil, err
		}
		if result[k] == nil {
			result[k] = make(map[string]core.ForeignKeyRef)
		}
		result[k][localCol] = core.ForeignKeyRef{
			SchemaName: refSchema,
			TableName:  refTable,
			ColumnName: refCol,
		}
	}
	return result, rows.Err()
}

// batchTableDDL builds the CREATE TABLE DDL of every table in the given
// schemas in one query, columns and constraints aggregated separately.
func (d *Dialect) batchTableDDL(ctx context.Context, db *sql.DB, schemas []string) (map[relKey]string, error) {
	const query = `
		WITH cols AS (
			SELECT
				n.nspname AS schema_name,
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
			  AND n.nspname = ANY($1::text[])
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
			cols.schema_name,
			cols.table_name,
			cols.columns_ddl ||
			CASE WHEN cons.constraints_ddl IS NOT NULL
				THEN E',\n\n' || cons.constraints_ddl
				ELSE ''
			END || E'\n);'
		FROM cols
		LEFT JOIN cons ON cons.table_oid = cols.table_oid;
	`

	rows, err := db.QueryContext(ctx, query, pq.Array(schemas))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[relKey]string)
	for rows.Next() {
		var k relKey
		var rawDDL string
		if err := rows.Scan(&k.schema, &k.name, &rawDDL); err != nil {
			return nil, err
		}
		result[k] = core.FormatTableDDL(rawDDL)
	}
	return result, rows.Err()
}

// GetViews returns the views of each schema in two queries.
func (d *Dialect) GetViews(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.Table, error) {
	const listQuery = `
		SELECT n.nspname, c.relname, pg_catalog.pg_get_viewdef(c.oid, true)
		FROM pg_catalog.pg_class AS c
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		WHERE n.nspname = ANY($1::text[]) AND c.relkind = 'v'
		ORDER BY n.nspname, c.relname ASC;
	`
	rows, err := db.QueryContext(ctx, listQuery, pq.Array(schemas))
	if err != nil {
		return nil, fmt.Errorf("failed to query views: %w", err)
	}

	type viewRow struct {
		key relKey
		def sql.NullString
	}
	var viewRows []viewRow
	for rows.Next() {
		var r viewRow
		if err := rows.Scan(&r.key.schema, &r.key.name, &r.def); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan view row: %w", err)
		}
		viewRows = append(viewRows, r)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if len(viewRows) == 0 {
		return map[string][]core.Table{}, nil
	}

	allColumns, err := d.batchColumns(ctx, db, schemas, []string{"v"})
	if err != nil {
		return nil, fmt.Errorf("failed to batch view columns: %w", err)
	}

	views := make(map[string][]core.Table)
	for _, r := range viewRows {
		definition := strings.TrimSpace(getString(r.def))
		ddl := ""
		if definition != "" {
			ddl = fmt.Sprintf("CREATE VIEW %s AS\n%s", r.key.name, definition)
		}
		views[r.key.schema] = append(views[r.key.schema], core.Table{
			Name:    r.key.name,
			Columns: allColumns[r.key],
			DDL:     ddl,
		})
	}
	return views, nil
}

// GetIndexes returns the indexes of each schema in two queries.
func (d *Dialect) GetIndexes(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.IndexInfo, error) {
	const listQuery = `
		SELECT n.nspname, i.relname, t.relname, pg_catalog.pg_get_indexdef(i.oid), i.oid::bigint
		FROM pg_catalog.pg_class AS i
		JOIN pg_catalog.pg_index AS ix ON ix.indexrelid = i.oid
		JOIN pg_catalog.pg_class AS t ON t.oid = ix.indrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = t.relnamespace
		WHERE n.nspname = ANY($1::text[])
		ORDER BY n.nspname, t.relname, i.relname;
	`
	rows, err := db.QueryContext(ctx, listQuery, pq.Array(schemas))
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}

	type indexRow struct {
		schema, name, table string
		ddl                 sql.NullString
		oid                 int64
	}
	var indexRows []indexRow
	for rows.Next() {
		var r indexRow
		if err := rows.Scan(&r.schema, &r.name, &r.table, &r.ddl, &r.oid); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}
		indexRows = append(indexRows, r)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if len(indexRows) == 0 {
		return map[string][]core.IndexInfo{}, nil
	}

	allCols, err := d.batchIndexColumns(ctx, db, schemas)
	if err != nil {
		return nil, fmt.Errorf("failed to batch index columns: %w", err)
	}

	indexes := make(map[string][]core.IndexInfo)
	for _, r := range indexRows {
		indexes[r.schema] = append(indexes[r.schema], core.IndexInfo{
			Name:      r.name,
			TableName: r.table,
			DDL:       getString(r.ddl),
			Columns:   allCols[r.oid],
		})
	}
	return indexes, nil
}

// batchIndexColumns fetches the columns of every index on a relation in the
// given schemas in one query, keyed by index OID.
func (d *Dialect) batchIndexColumns(ctx context.Context, db *sql.DB, schemas []string) (map[int64][]core.IndexColumnInfo, error) {
	const query = `
		SELECT
			ix.indexrelid::bigint,
			COALESCE(a.attname, ''),
			cols.ordinality,
			COALESCE(coll.collname, ''),
			(ix.indoption[cols.ordinality - 1] & 1) = 1
		FROM pg_catalog.pg_index AS ix
		JOIN pg_catalog.pg_class AS t ON t.oid = ix.indrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = t.relnamespace
		JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS cols(attnum, ordinality) ON true
		LEFT JOIN pg_catalog.pg_attribute AS a ON a.attrelid = t.oid AND a.attnum = cols.attnum
		LEFT JOIN pg_catalog.pg_collation AS coll ON coll.oid = ix.indcollation[cols.ordinality - 1]
		WHERE n.nspname = ANY($1::text[])
		  AND cols.attnum <> 0
		ORDER BY ix.indexrelid, cols.ordinality;
	`
	rows, err := db.QueryContext(ctx, query, pq.Array(schemas))
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

// GetTriggers returns the triggers of each schema in one query.
func (d *Dialect) GetTriggers(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.TriggerInfo, error) {
	const query = `
		SELECT n.nspname, tg.tgname, tbl.relname, pg_catalog.pg_get_triggerdef(tg.oid, true)
		FROM pg_catalog.pg_trigger AS tg
		JOIN pg_catalog.pg_class AS tbl ON tbl.oid = tg.tgrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = tbl.relnamespace
		WHERE NOT tg.tgisinternal AND n.nspname = ANY($1::text[])
		ORDER BY n.nspname, tg.tgname ASC;
	`
	rows, err := db.QueryContext(ctx, query, pq.Array(schemas))
	if err != nil {
		return nil, fmt.Errorf("failed to query triggers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	triggers := make(map[string][]core.TriggerInfo)
	for rows.Next() {
		var schema, name, table string
		var ddl sql.NullString
		if err := rows.Scan(&schema, &name, &table, &ddl); err != nil {
			return nil, fmt.Errorf("failed to scan trigger row: %w", err)
		}
		triggers[schema] = append(triggers[schema], core.TriggerInfo{Name: name, TableName: table, DDL: getString(ddl)})
	}
	return triggers, rows.Err()
}

// GetStats returns the table row counts and index scan counts of each schema
// in two queries.
func (d *Dialect) GetStats(ctx context.Context, db *sql.DB, schemas []string) (map[string]core.TableStats, error) {
	stats := make(map[string]core.TableStats)
	queries := []struct{ sql, what string }{
		{`SELECT schemaname, relname, n_live_tup::text
		  FROM pg_catalog.pg_stat_user_tables
		  WHERE schemaname = ANY($1::text[]);`, "table"},
		{`SELECT schemaname, indexrelname, COALESCE(idx_scan, 0)::text
		  FROM pg_catalog.pg_stat_user_indexes
		  WHERE schemaname = ANY($1::text[]);`, "index"},
	}
	for _, q := range queries {
		rows, err := db.QueryContext(ctx, q.sql, pq.Array(schemas))
		if err != nil {
			return nil, fmt.Errorf("failed to query %s statistics: %w", q.what, err)
		}
		for rows.Next() {
			var schema, name, val string
			if err := rows.Scan(&schema, &name, &val); err != nil {
				_ = rows.Close()
				return nil, err
			}
			if stats[schema] == nil {
				stats[schema] = make(core.TableStats)
			}
			stats[schema][name] = val
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return stats, nil
}

// GetTypes returns the scalar and enum types of each schema in one query.
func (d *Dialect) GetTypes(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.Type, error) {
	const q = `
SELECT n.nspname, t.typname, t.typtype::text,
       COALESCE(pg_catalog.format_type(t.oid, NULL), t.typname::text),
       obj_description(t.oid, 'pg_type'),
       (SELECT string_agg(e.enumlabel, E'\x01' ORDER BY e.enumsortorder)
        FROM pg_catalog.pg_enum e WHERE e.enumtypid = t.oid)
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
WHERE t.typisdefined
  AND n.nspname = ANY($1::text[])
  AND t.typcategory NOT IN ('p', 'x')
  AND t.typelem = 0
  AND t.typname NOT LIKE 'pg_%'
ORDER BY n.nspname, t.typname
`
	rows, err := db.QueryContext(ctx, q, pq.Array(schemas))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string][]core.Type)
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
		out[schemaName] = append(out[schemaName], ct)
	}
	return out, rows.Err()
}

// GetFunctions returns the callable routines of each user schema in one query.
// System schemas are skipped: pg_catalog builtins come from GetCatalogSchema.
func (d *Dialect) GetFunctions(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.Function, error) {
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
	rows, err := db.QueryContext(ctx, q, pq.Array(schemas))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string][]core.Function)
	for rows.Next() {
		var oid int64
		var schema, name, args, result, kind string
		var description sql.NullString
		if err := rows.Scan(&oid, &schema, &name, &args, &result, &kind, &description); err != nil {
			return nil, err
		}
		out[schema] = append(out[schema], core.Function{
			OID: oid, Schema: schema, Name: name, Args: args,
			Result: result, Kind: kind,
			Description: strings.TrimSpace(getString(description)),
		})
	}
	return out, rows.Err()
}

// GetCatalogSchema returns pg_catalog as a fully-populated Schema.
// Called lazily for lint/completion, not during regular schema load.
func (d *Dialect) GetCatalogSchema(ctx context.Context, db *sql.DB) (*core.Schema, error) {
	catalog := []string{"pg_catalog"}
	tables, err := d.GetTables(ctx, db, catalog)
	if err != nil {
		return nil, err
	}
	views, err := d.GetViews(ctx, db, catalog)
	if err != nil {
		return nil, err
	}
	types, err := d.GetTypes(ctx, db, catalog)
	if err != nil {
		return nil, err
	}
	funcs, err := loadPGCatalogBuiltinsMatchingList(ctx, db, nonPgBuiltinNames())
	if err != nil {
		return nil, err
	}
	return &core.Schema{
		Name:      "pg_catalog",
		Tables:    tables["pg_catalog"],
		Views:     views["pg_catalog"],
		Types:     types["pg_catalog"],
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
