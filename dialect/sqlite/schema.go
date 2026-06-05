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

// GetTables returns all tables in the SQLite database.
// The schema parameter is ignored as SQLite only has "main".
func (d *Dialect) GetTables(ctx context.Context, db *sql.DB, schema string) ([]core.Table, error) {
	query := `
		SELECT name, sql
		FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name ASC;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tables []core.Table
	for rows.Next() {
		var tableName string
		var tableDDL sql.NullString
		if err := rows.Scan(&tableName, &tableDDL); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		columns, err := d.discoverColumns(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to discover columns for table %s: %w", tableName, err)
		}

		primaryKey, _ := d.discoverPrimaryKey(ctx, db, tableName)
		foreignKeys, _ := d.discoverForeignKeys(ctx, db, tableName)
		core.EnrichColumnsWithConstraints(&columns, primaryKey, foreignKeys)

		tables = append(tables, core.Table{
			Name:       tableName,
			Columns:    columns,
			PrimaryKey: primaryKey,
			DDL:        getString(tableDDL),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating table rows: %w", err)
	}

	return tables, nil
}

// discoverPrimaryKey returns the column names that make up the primary key for a table
func (d *Dialect) discoverPrimaryKey(ctx context.Context, db *sql.DB, tableName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA table_info(%q)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query primary key: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pkColumn struct {
		name     string
		position int
	}
	var pkColumns []pkColumn

	for rows.Next() {
		var (
			columnID     int
			columnName   string
			columnType   string
			isNotNull    int
			defaultVal   sql.NullString
			isPrimaryKey int
		)

		if err := rows.Scan(&columnID, &columnName, &columnType, &isNotNull, &defaultVal, &isPrimaryKey); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		if isPrimaryKey > 0 {
			pkColumns = append(pkColumns, pkColumn{name: columnName, position: isPrimaryKey})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating columns: %w", err)
	}

	if len(pkColumns) == 0 {
		return nil, nil
	}

	primaryKey := make([]string, len(pkColumns))
	for _, col := range pkColumns {
		primaryKey[col.position-1] = col.name
	}

	return primaryKey, nil
}

// GetViews returns all views in the SQLite database.
// The schema parameter is ignored as SQLite only has "main".
func (d *Dialect) GetViews(ctx context.Context, db *sql.DB, schema string) ([]core.Table, error) {
	query := `
		SELECT name, sql
		FROM sqlite_master
		WHERE type = 'view' AND name NOT LIKE 'sqlite_%'
		ORDER BY name ASC;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var views []core.Table
	for rows.Next() {
		var viewName string
		var viewDDL sql.NullString
		if err := rows.Scan(&viewName, &viewDDL); err != nil {
			return nil, fmt.Errorf("failed to scan view name: %w", err)
		}

		columns, err := d.discoverColumns(ctx, db, viewName)
		if err != nil {
			return nil, fmt.Errorf("failed to discover columns for view %s: %w", viewName, err)
		}

		views = append(views, core.Table{
			Name:    viewName,
			Columns: columns,
			DDL:     getString(viewDDL),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating view rows: %w", err)
	}

	return views, nil
}

// discoverColumns discovers all columns for a table or view using PRAGMA table_info
func (d *Dialect) discoverColumns(ctx context.Context, db *sql.DB, relationName string) ([]core.Column, error) {
	query := fmt.Sprintf("PRAGMA table_info(%q)", relationName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var columns []core.Column
	for rows.Next() {
		var (
			columnID     int
			columnName   string
			columnType   string
			isNotNull    int
			defaultVal   sql.NullString
			isPrimaryKey int
		)

		if err := rows.Scan(&columnID, &columnName, &columnType, &isNotNull, &defaultVal, &isPrimaryKey); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		nullable := isNotNull == 0 && isPrimaryKey == 0

		col := core.Column{
			Name:         columnName,
			Type:         columnType,
			Nullable:     nullable,
			IsPrimaryKey: isPrimaryKey > 0,
		}
		if defaultVal.Valid && defaultVal.String != "" {
			col.Default = &defaultVal.String
		}
		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating column rows: %w", err)
	}

	return columns, nil
}

// sqliteDefaultSchema is the default schema name for SQLite (PRAGMA database_list reports "main").
const sqliteDefaultSchema = "main"

// discoverForeignKeys returns a map of local column name to referenced (schema, table, column) for the given table.
func (d *Dialect) discoverForeignKeys(ctx context.Context, db *sql.DB, relationName string) (map[string]core.ForeignKeyRef, error) {
	query := fmt.Sprintf("PRAGMA foreign_key_list(%q)", relationName)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]core.ForeignKeyRef)
	for rows.Next() {
		var id, seq int
		var refTable, fromCol, toCol string
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key row: %w", err)
		}
		result[fromCol] = core.ForeignKeyRef{
			SchemaName: sqliteDefaultSchema,
			TableName:  refTable,
			ColumnName: toCol,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating foreign key rows: %w", err)
	}

	return result, nil
}

// GetIndexes returns all indexes in the SQLite database.
// The schema parameter is ignored as SQLite only has "main".
func (d *Dialect) GetIndexes(ctx context.Context, db *sql.DB, schema string) ([]core.IndexInfo, error) {
	rows, err := db.QueryContext(ctx, `
        SELECT name, tbl_name, sql
        FROM sqlite_master
        WHERE type = 'index'
        ORDER BY tbl_name, name;
    `)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var indexes []core.IndexInfo
	for rows.Next() {
		var indexName, tableName, indexSQL sql.NullString
		if err := rows.Scan(&indexName, &tableName, &indexSQL); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		if !indexName.Valid {
			continue
		}

		indexInfoRows, err := db.QueryContext(ctx, `
			SELECT seqno, cid, "key", name, coll, desc
			FROM pragma_index_xinfo(?)
			ORDER BY seqno ASC;
		`, indexName.String)
		if err != nil {
			return nil, fmt.Errorf("failed to query index columns: %w", err)
		}
		defer func() { _ = indexInfoRows.Close() }()

		var columnInfos []core.IndexColumnInfo
		for indexInfoRows.Next() {
			var seqNo, colID, isKey, isDesc sql.NullInt64
			var colName, colCollation sql.NullString

			if err := indexInfoRows.Scan(&seqNo, &colID, &isKey, &colName, &colCollation, &isDesc); err != nil {
				return nil, fmt.Errorf("failed to scan index column: %w", err)
			}

			if !colName.Valid {
				continue
			}

			position := int(getInt64(seqNo)) + 1
			descending := getInt64(isDesc) != 0

			columnInfos = append(columnInfos, core.IndexColumnInfo{
				Name:       colName.String,
				Position:   position,
				Collation:  getString(colCollation),
				Descending: descending,
			})
		}

		if err := indexInfoRows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating index column rows: %w", err)
		}

		indexes = append(indexes, core.IndexInfo{
			Name:      indexName.String,
			TableName: getString(tableName),
			DDL:       getString(indexSQL),
			Columns:   columnInfos,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating index rows: %w", err)
	}

	return indexes, nil
}

// GetTriggers returns all triggers in the SQLite database.
// The schema parameter is ignored as SQLite only has "main".
func (d *Dialect) GetTriggers(ctx context.Context, db *sql.DB, schema string) ([]core.TriggerInfo, error) {
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

	return triggers, nil
}

// GetStats returns statistics for tables and indexes in the SQLite database.
// The schema parameter is ignored as SQLite only has "main".
func (d *Dialect) GetStats(ctx context.Context, db *sql.DB, schema string) (core.TableStats, error) {
	_, err := db.ExecContext(ctx, "ANALYZE")
	if err != nil {
		return nil, fmt.Errorf("failed to run ANALYZE: %w", err)
	}

	query := `
        SELECT tbl, idx, stat
        FROM sqlite_stat1
    `

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return make(core.TableStats), nil
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

	return stats, nil
}

// Helper functions for SQL nullable types
func getString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func getInt64(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}

// GetTypes returns nil; SQLite has no user-defined types per schema.
func (d *Dialect) GetTypes(_ context.Context, _ *sql.DB, _ string) ([]core.Type, error) {
	return nil, nil
}

// GetFunctions returns nil; SQLite has no user-defined functions per schema.
func (d *Dialect) GetFunctions(_ context.Context, _ *sql.DB, _ string) ([]core.Function, error) {
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
