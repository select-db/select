package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	core "github.com/selectDb/dialect/core"
)

// DefaultSchemaName implements core.SQLDialect.DefaultSchemaName.
func (d *Dialect) DefaultSchemaName() string { return "main" }

// GetCurrentSchema returns the current active schema for the SQLite database.
// SQLite doesn't have real schemas, so this always returns "main".
func (d *Dialect) GetCurrentSchema(_ context.Context, _ *sql.DB) (string, error) {
	return "main", nil
}

// GetSchemas returns all schemas in the SQLite database.
// SQLite doesn't have real schemas, so this always returns ["main"].
func (d *Dialect) GetSchemas(_ context.Context, _ *sql.DB) ([]string, error) {
	return []string{"main"}, nil
}

// The Get methods answer for "main", SQLite's only schema, whatever schemas holds.

// GetTables returns every table with its columns, primary key and foreign keys.
func (d *Dialect) GetTables(ctx context.Context, db *sql.DB, _ []string) (map[string][]core.Table, error) {
	tables, err := d.relationsWithColumns(ctx, db, "table")
	if err != nil {
		return nil, err
	}
	foreignKeys, _ := d.foreignKeys(ctx, db)
	for i := range tables {
		core.EnrichColumnsWithConstraints(&tables[i].Columns, tables[i].PrimaryKey, foreignKeys[tables[i].Name])
	}
	return map[string][]core.Table{sqliteDefaultSchema: tables}, nil
}

// GetViews returns all views in the SQLite database.
func (d *Dialect) GetViews(ctx context.Context, db *sql.DB, _ []string) (map[string][]core.Table, error) {
	views, err := d.relationsWithColumns(ctx, db, "view")
	return map[string][]core.Table{sqliteDefaultSchema: views}, err
}

// relationsWithColumns reads every table or view (kind) and its columns in one
// query, so a schema load costs the same whatever the number of relations.
func (d *Dialect) relationsWithColumns(ctx context.Context, db *sql.DB, kind string) ([]core.Table, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.name, m.sql, p.name, p.type, p."notnull", p.dflt_value, p.pk
		FROM sqlite_master m JOIN pragma_table_info(m.name, 'main') p
		WHERE m.type = ? AND m.name NOT LIKE 'sqlite_%'
		ORDER BY m.name, p.cid`, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to query %ss: %w", kind, err)
	}
	defer func() { _ = rows.Close() }()

	var relations []core.Table
	for rows.Next() {
		var (
			relName, colName, colType string
			ddl, defaultVal           sql.NullString
			notNull, pk               int
		)
		if err := rows.Scan(&relName, &ddl, &colName, &colType, &notNull, &defaultVal, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan %s column: %w", kind, err)
		}
		if len(relations) == 0 || relations[len(relations)-1].Name != relName {
			relations = append(relations, core.Table{Name: relName, DDL: ddl.String})
		}
		col := core.Column{
			Name:         colName,
			Type:         colType,
			Nullable:     notNull == 0 && pk == 0,
			IsPrimaryKey: pk > 0,
		}
		if defaultVal.Valid && defaultVal.String != "" {
			col.Default = &defaultVal.String
		}
		rel := &relations[len(relations)-1]
		rel.Columns = append(rel.Columns, col)
		// pk is the column's 1-based position in the key.
		if pk > 0 {
			for len(rel.PrimaryKey) < pk {
				rel.PrimaryKey = append(rel.PrimaryKey, "")
			}
			rel.PrimaryKey[pk-1] = colName
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating %s rows: %w", kind, err)
	}
	return relations, nil
}

// sqliteDefaultSchema is the default schema name for SQLite (PRAGMA database_list reports "main").
const sqliteDefaultSchema = "main"

// foreignKeys maps each table to its local column -> referenced column. The
// referenced column is "" when the key names the parent's primary key.
func (d *Dialect) foreignKeys(ctx context.Context, db *sql.DB) (map[string]map[string]core.ForeignKeyRef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.name, f."from", f."table", f."to"
		FROM sqlite_master m JOIN pragma_foreign_key_list(m.name, 'main') f
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]map[string]core.ForeignKeyRef)
	for rows.Next() {
		var table, fromCol, refTable string
		var toCol sql.NullString
		if err := rows.Scan(&table, &fromCol, &refTable, &toCol); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key row: %w", err)
		}
		if result[table] == nil {
			result[table] = make(map[string]core.ForeignKeyRef)
		}
		result[table][fromCol] = core.ForeignKeyRef{
			SchemaName: sqliteDefaultSchema,
			TableName:  refTable,
			ColumnName: toCol.String,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating foreign key rows: %w", err)
	}
	return result, nil
}

// GetIndexes returns all indexes in the SQLite database.
func (d *Dialect) GetIndexes(ctx context.Context, db *sql.DB, _ []string) (map[string][]core.IndexInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.name, m.tbl_name, m.sql, x.seqno, x.name, x.coll, x."desc"
		FROM sqlite_master m LEFT JOIN pragma_index_xinfo(m.name, 'main') x
		WHERE m.type = 'index'
		ORDER BY m.tbl_name, m.name, x.seqno`)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var indexes []core.IndexInfo
	for rows.Next() {
		var indexName, tableName, indexSQL, colName, colCollation sql.NullString
		var seqNo, isDesc sql.NullInt64
		if err := rows.Scan(&indexName, &tableName, &indexSQL, &seqNo, &colName, &colCollation, &isDesc); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}
		if !indexName.Valid {
			continue
		}
		if len(indexes) == 0 || indexes[len(indexes)-1].Name != indexName.String {
			indexes = append(indexes, core.IndexInfo{
				Name:      indexName.String,
				TableName: tableName.String,
				DDL:       indexSQL.String,
			})
		}
		// An expression or the rowid has no column name.
		if !colName.Valid {
			continue
		}
		idx := &indexes[len(indexes)-1]
		idx.Columns = append(idx.Columns, core.IndexColumnInfo{
			Name:       colName.String,
			Position:   int(seqNo.Int64) + 1,
			Collation:  colCollation.String,
			Descending: isDesc.Int64 != 0,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating index rows: %w", err)
	}
	return map[string][]core.IndexInfo{sqliteDefaultSchema: indexes}, nil
}

// GetTriggers returns all triggers in the SQLite database.
func (d *Dialect) GetTriggers(ctx context.Context, db *sql.DB, _ []string) (map[string][]core.TriggerInfo, error) {
	query := `
		SELECT name, tbl_name, sql
		FROM sqlite_master
		WHERE type = 'trigger'
		ORDER BY name ASC;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query triggers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var triggers []core.TriggerInfo
	for rows.Next() {
		var triggerName, tableName, triggerSQL string
		if err := rows.Scan(&triggerName, &tableName, &triggerSQL); err != nil {
			return nil, fmt.Errorf("failed to scan trigger: %w", err)
		}

		triggers = append(triggers, core.TriggerInfo{
			Name:      triggerName,
			TableName: tableName,
			DDL:       triggerSQL,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating trigger rows: %w", err)
	}

	return map[string][]core.TriggerInfo{sqliteDefaultSchema: triggers}, nil
}

// GetStats returns what the last ANALYZE left in sqlite_stat1, if anything:
// reading a schema never writes to the database.
func (d *Dialect) GetStats(ctx context.Context, db *sql.DB, _ []string) (map[string]core.TableStats, error) {
	query := `
        SELECT tbl, idx, stat
        FROM sqlite_stat1
    `

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = rows.Close() }()

	stats := make(core.TableStats)
	for rows.Next() {
		var tableName string
		var indexName sql.NullString
		var statValue string

		if err := rows.Scan(&tableName, &indexName, &statValue); err != nil {
			return nil, fmt.Errorf("failed to scan statistic: %w", err)
		}

		if !indexName.Valid || indexName.String == "" {
			stats[tableName] = statValue
		} else {
			stats[indexName.String] = statValue
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating statistic rows: %w", err)
	}

	return map[string]core.TableStats{sqliteDefaultSchema: stats}, nil
}

// GetTypes returns nil; SQLite has no user-defined types per schema.
func (d *Dialect) GetTypes(context.Context, *sql.DB, []string) (map[string][]core.Type, error) {
	return nil, nil
}

// GetFunctions returns nil; SQLite has no user-defined functions per schema.
func (d *Dialect) GetFunctions(context.Context, *sql.DB, []string) (map[string][]core.Function, error) {
	return nil, nil
}

// GetCatalogSchema returns a synthetic schema containing SQLite built-in type affinities
// and built-in functions. SQLite has no queryable system catalog, so these are hardcoded.
func (d *Dialect) GetCatalogSchema(_ context.Context, _ *sql.DB) (*core.Schema, error) {
	return &core.Schema{
		Name:      "sqlite_builtin",
		Types:     sqliteTypes,
		Functions: sqliteFunctions,
	}, nil
}

// GetSettings returns SQLite pragma names from pragma_pragma_list. Values and
// descriptions are not exposed there; each pragma is queried individually.
func (d *Dialect) GetSettings(ctx context.Context, db *sql.DB) ([]core.Setting, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_pragma_list ORDER BY name;`)
	if err != nil {
		return nil, fmt.Errorf("failed to query pragma_pragma_list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.Setting
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan pragma row: %w", err)
		}
		out = append(out, core.Setting{Name: name})
	}
	return out, rows.Err()
}
