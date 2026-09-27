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

// schemaQuery reads the user schemas and pg_catalog in one round trip. It takes
// no bind parameters so lib/pq sends it with the simple protocol; the builtin
// function names are inlined where :builtins stands.
const schemaQuery = `
	WITH ns AS (
		SELECT oid, nspname, nspname = 'pg_catalog' AS catalog
		FROM pg_catalog.pg_namespace
		WHERE (nspname NOT LIKE 'pg_%' AND nspname <> 'information_schema') OR nspname = 'pg_catalog'
	),
	table_cols AS (
		SELECT c.oid,
			'CREATE TABLE ' || quote_ident(n.nspname) || '.' || quote_ident(c.relname) || E' (\n' ||
			string_agg(
				'    ' || quote_ident(a.attname) || ' ' || pg_catalog.format_type(a.atttypid, a.atttypmod) ||
				CASE WHEN a.attnotnull THEN ' NOT NULL' ELSE '' END ||
				CASE WHEN ad.adbin IS NOT NULL THEN ' DEFAULT ' || pg_catalog.pg_get_expr(ad.adbin, ad.adrelid) ELSE '' END,
				E',\n' ORDER BY a.attnum) AS ddl
		FROM pg_catalog.pg_class c
		JOIN ns n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
		WHERE c.relkind IN ('r', 'p')
		GROUP BY n.nspname, c.relname, c.oid
	),
	table_cons AS (
		SELECT con.conrelid AS oid,
			string_agg('    CONSTRAINT ' || quote_ident(con.conname) || ' ' || pg_catalog.pg_get_constraintdef(con.oid, true),
				E',\n' ORDER BY CASE con.contype WHEN 'p' THEN 1 WHEN 'u' THEN 2 WHEN 'c' THEN 3 WHEN 'f' THEN 4 ELSE 5 END, con.conname) AS ddl
		FROM pg_catalog.pg_constraint con
		WHERE con.conrelid IN (SELECT oid FROM table_cols)
		GROUP BY con.conrelid
	)
	SELECT 'schema', json_build_object('name', nspname, 'catalog', catalog) FROM ns
	UNION ALL
	SELECT 'current', json_build_object('name', COALESCE(
		NULLIF(current_schema(), ''),
		NULLIF(btrim(btrim(split_part(current_setting('search_path'), ',', 1), '"'), E' \t\n\r\x0b\f'), ''),
		'public'))
	UNION ALL
	SELECT 'relation', json_build_object('schema', n.nspname, 'name', c.relname, 'kind', 'table',
		'description', COALESCE(d.description, ''),
		'ddl', COALESCE(tc.ddl || COALESCE(E',\n\n' || cons.ddl, '') || E'\n);', ''))
	FROM pg_catalog.pg_class c
	JOIN ns n ON n.oid = c.relnamespace
	LEFT JOIN pg_catalog.pg_description d
		ON d.objoid = c.oid AND d.classoid = 'pg_catalog.pg_class'::regclass AND d.objsubid = 0
	LEFT JOIN table_cols tc ON tc.oid = c.oid
	LEFT JOIN table_cons cons ON cons.oid = c.oid
	WHERE c.relkind IN ('r', 'p')
	UNION ALL
	SELECT 'relation', json_build_object('schema', n.nspname, 'name', c.relname, 'kind', 'view',
		'ddl', COALESCE('CREATE VIEW ' || c.relname || E' AS\n' ||
			NULLIF(btrim(pg_catalog.pg_get_viewdef(c.oid, true), E' \t\n\r\x0b\f'), ''), ''))
	FROM pg_catalog.pg_class c
	JOIN ns n ON n.oid = c.relnamespace
	WHERE c.relkind = 'v'
	UNION ALL
	SELECT 'column', json_build_object('schema', n.nspname, 'table', c.relname, 'name', a.attname,
		'type', pg_catalog.format_type(a.atttypid, a.atttypmod), 'nullable', NOT a.attnotnull,
		'default', NULLIF(pg_catalog.pg_get_expr(ad.adbin, ad.adrelid), ''), 'position', a.attnum,
		'description', COALESCE(d.description, ''))
	FROM pg_catalog.pg_attribute a
	JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
	JOIN ns n ON n.oid = c.relnamespace
	LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
	-- A join, not col_description per row: one pass over pg_description.
	LEFT JOIN pg_catalog.pg_description d
		ON d.objoid = c.oid AND d.classoid = 'pg_catalog.pg_class'::regclass AND d.objsubid = a.attnum
	WHERE c.relkind IN ('r', 'p', 'v') AND a.attnum > 0 AND NOT a.attisdropped
	UNION ALL
	SELECT 'primary_key', json_build_object('schema', n.nspname, 'table', c.relname, 'column', a.attname,
		'position', array_position(con.conkey, a.attnum))
	FROM pg_catalog.pg_constraint con
	JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
	JOIN ns n ON n.oid = c.relnamespace
	JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
	WHERE con.contype = 'p'
	UNION ALL
	SELECT 'foreign_key', json_build_object('schema', n.nspname, 'table', c.relname, 'column', a.attname,
		'refSchema', ref_n.nspname, 'refTable', ref_c.relname, 'refColumn', ref_a.attname)
	FROM pg_catalog.pg_constraint con
	JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
	JOIN ns n ON n.oid = c.relnamespace
	JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord) ON true
	JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid AND a.attnum = k.attnum AND NOT a.attisdropped AND a.attnum > 0
	JOIN LATERAL unnest(con.confkey) WITH ORDINALITY AS r(attnum, ord) ON r.ord = k.ord
	JOIN pg_catalog.pg_class ref_c ON ref_c.oid = con.confrelid
	JOIN pg_catalog.pg_namespace ref_n ON ref_n.oid = ref_c.relnamespace
	JOIN pg_catalog.pg_attribute ref_a ON ref_a.attrelid = ref_c.oid AND ref_a.attnum = r.attnum AND NOT ref_a.attisdropped AND ref_a.attnum > 0
	WHERE con.contype = 'f'
	UNION ALL
	SELECT 'index', json_build_object('schema', n.nspname, 'table', t.relname, 'name', i.relname,
		'ddl', COALESCE(pg_catalog.pg_get_indexdef(i.oid), ''))
	FROM pg_catalog.pg_index ix
	JOIN pg_catalog.pg_class i ON i.oid = ix.indexrelid
	JOIN pg_catalog.pg_class t ON t.oid = ix.indrelid
	JOIN ns n ON n.oid = t.relnamespace AND NOT n.catalog
	UNION ALL
	SELECT 'index_column', json_build_object('schema', n.nspname, 'table', t.relname, 'index', i.relname,
		'name', COALESCE(a.attname, ''), 'position', cols.ordinality,
		'collation', COALESCE(coll.collname, ''), 'descending', (ix.indoption[cols.ordinality - 1] & 1) = 1)
	FROM pg_catalog.pg_index ix
	JOIN pg_catalog.pg_class i ON i.oid = ix.indexrelid
	JOIN pg_catalog.pg_class t ON t.oid = ix.indrelid
	JOIN ns n ON n.oid = t.relnamespace AND NOT n.catalog
	JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS cols(attnum, ordinality) ON true
	LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = t.oid AND a.attnum = cols.attnum
	LEFT JOIN pg_catalog.pg_collation coll ON coll.oid = ix.indcollation[cols.ordinality - 1]
	WHERE cols.attnum <> 0
	UNION ALL
	SELECT 'trigger', json_build_object('schema', n.nspname, 'table', c.relname, 'name', tg.tgname,
		'ddl', COALESCE(pg_catalog.pg_get_triggerdef(tg.oid, true), ''))
	FROM pg_catalog.pg_trigger tg
	JOIN pg_catalog.pg_class c ON c.oid = tg.tgrelid
	JOIN ns n ON n.oid = c.relnamespace AND NOT n.catalog
	WHERE NOT tg.tgisinternal
	UNION ALL
	SELECT 'stat', json_build_object('schema', s.schemaname, 'name', s.relname, 'value', s.n_live_tup::text)
	FROM pg_catalog.pg_stat_user_tables s
	JOIN ns n ON n.nspname = s.schemaname AND NOT n.catalog
	UNION ALL
	SELECT 'stat', json_build_object('schema', s.schemaname, 'name', s.indexrelname, 'value', COALESCE(s.idx_scan, 0)::text)
	FROM pg_catalog.pg_stat_user_indexes s
	JOIN ns n ON n.nspname = s.schemaname AND NOT n.catalog
	UNION ALL
	SELECT 'type', json_build_object('schema', n.nspname, 'name', t.typname, 'kind', t.typtype,
		'display', COALESCE(pg_catalog.format_type(t.oid, NULL), t.typname::text),
		'description', COALESCE(btrim(pg_catalog.obj_description(t.oid, 'pg_type'), E' \t\n\r\x0b\f'), ''),
		'enumLabels', (SELECT json_agg(e.enumlabel ORDER BY e.enumsortorder) FROM pg_catalog.pg_enum e WHERE e.enumtypid = t.oid))
	FROM pg_catalog.pg_type t
	JOIN ns n ON n.oid = t.typnamespace
	WHERE t.typisdefined AND t.typcategory NOT IN ('p', 'x') AND t.typelem = 0 AND t.typname NOT LIKE 'pg_%'
	UNION ALL
	SELECT 'function', json_build_object('schema', n.nspname, 'name', p.proname, 'oid', p.oid::bigint,
		'args', pg_catalog.pg_get_function_identity_arguments(p.oid),
		'result', pg_catalog.pg_get_function_result(p.oid), 'kind', p.prokind,
		'description', COALESCE(btrim(pg_catalog.obj_description(p.oid, 'pg_proc'), E' \t\n\r\x0b\f'), ''))
	FROM pg_catalog.pg_proc p
	JOIN ns n ON n.oid = p.pronamespace
	WHERE NOT n.catalog OR p.proname = ANY(:builtins)
	UNION ALL
	SELECT 'setting', json_build_object('name', name, 'value', setting, 'description', COALESCE(short_desc, ''))
	FROM pg_catalog.pg_settings`

// ReadSchema reads the user schemas and pg_catalog in one query.
func (d *Dialect) ReadSchema(ctx context.Context, db *sql.DB) (*core.Metadata, error) {
	names, err := pq.Array(nonPgBuiltinNames()).Value()
	if err != nil {
		return nil, fmt.Errorf("failed to encode builtin names: %w", err)
	}
	query := strings.Replace(schemaQuery, ":builtins", pq.QuoteLiteral(names.(string))+"::text[]", 1)

	schemaRows, err := core.QuerySchemaRows(ctx, db, query)
	if err != nil {
		return nil, err
	}
	for i, rel := range schemaRows.Relations {
		if rel.Kind == "table" {
			schemaRows.Relations[i].DDL = core.FormatTableDDL(rel.DDL)
		}
	}
	return schemaRows.Metadata(), nil
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
