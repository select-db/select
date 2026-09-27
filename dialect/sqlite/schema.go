package sqlite

import (
	"context"
	"database/sql"

	core "github.com/selectDb/dialect/core"
)

// DefaultSchemaName implements core.SQLDialect.DefaultSchemaName.
func (d *Dialect) DefaultSchemaName() string { return "main" }

// sqliteDefaultSchema is the default schema name for SQLite (PRAGMA database_list reports "main").
const sqliteDefaultSchema = "main"

// schemaQuery reads everything but the statistics in one round trip.
// sqlite_stat1 exists only after an ANALYZE, so has_stat1 says whether to read it.
const schemaQuery = `
	SELECT 'schema', json_object('name', 'main', 'catalog', json('false'))
	UNION ALL SELECT 'schema', json_object('name', 'sqlite_builtin', 'catalog', json('true'))
	UNION ALL SELECT 'current', json_object('name', 'main')
	UNION ALL
	SELECT 'relation', json_object('schema', 'main', 'name', m.name, 'kind', m.type, 'ddl', COALESCE(m.sql, ''))
	FROM sqlite_master m
	WHERE m.type IN ('table', 'view') AND m.name NOT LIKE 'sqlite_%'
	UNION ALL
	SELECT 'column', json_object(
		'schema', 'main', 'table', m.name, 'name', p.name, 'type', p.type,
		'nullable', json(CASE WHEN p."notnull" = 0 AND p.pk = 0 THEN 'true' ELSE 'false' END),
		'default', NULLIF(p.dflt_value, ''), 'position', p.cid)
	FROM sqlite_master m JOIN pragma_table_info(m.name, 'main') p
	WHERE m.type IN ('table', 'view') AND m.name NOT LIKE 'sqlite_%'
	UNION ALL
	SELECT 'primary_key', json_object('schema', 'main', 'table', m.name, 'column', p.name, 'position', p.pk)
	FROM sqlite_master m JOIN pragma_table_info(m.name, 'main') p
	WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%' AND p.pk > 0
	UNION ALL
	SELECT 'foreign_key', json_object(
		'schema', 'main', 'table', m.name, 'column', f."from",
		'refSchema', 'main', 'refTable', f."table", 'refColumn', COALESCE(f."to", ''))
	FROM sqlite_master m JOIN pragma_foreign_key_list(m.name, 'main') f
	WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'
	UNION ALL
	SELECT 'index', json_object('schema', 'main', 'table', m.tbl_name, 'name', m.name, 'ddl', COALESCE(m.sql, ''))
	FROM sqlite_master m
	WHERE m.type = 'index'
	UNION ALL
	SELECT 'index_column', json_object(
		'schema', 'main', 'table', m.tbl_name, 'index', m.name, 'name', COALESCE(x.name, ''),
		'position', x.seqno + 1, 'collation', COALESCE(x.coll, ''),
		'descending', json(CASE WHEN x."desc" THEN 'true' ELSE 'false' END))
	FROM sqlite_master m JOIN pragma_index_xinfo(m.name, 'main') x
	WHERE m.type = 'index'
	UNION ALL
	SELECT 'trigger', json_object('schema', 'main', 'table', tbl_name, 'name', name, 'ddl', COALESCE(sql, ''))
	FROM sqlite_master
	WHERE type = 'trigger'
	UNION ALL
	SELECT 'setting', json_object('name', name) FROM pragma_pragma_list
	UNION ALL
	SELECT 'has_stat1', '{}' FROM sqlite_master WHERE name = 'sqlite_stat1'`

// ReadSchema reads main in one query, plus one for sqlite_stat1 when an
// ANALYZE left it. Reading a schema never writes to the database.
func (d *Dialect) ReadSchema(ctx context.Context, db *sql.DB) (*core.Metadata, error) {
	rows, err := core.QuerySchemaRows(ctx, db, schemaQuery)
	if err != nil {
		return nil, err
	}
	if rows.Other["has_stat1"] != nil {
		stats, err := core.QuerySchemaRows(ctx, db, `
			SELECT 'stat', json_object('schema', 'main', 'name', COALESCE(NULLIF(idx, ''), tbl), 'value', stat)
			FROM sqlite_stat1`)
		if err != nil {
			return nil, err
		}
		rows.Stats = stats.Stats
	}
	meta := rows.Metadata()
	catalog := &meta.Schemas[len(meta.Schemas)-1]
	catalog.Types, catalog.Functions = sqliteTypes, sqliteFunctions
	return meta, nil
}
