package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	core "github.com/selectDb/dialect/core"
)

// DefaultSchemaName implements core.SQLDialect.DefaultSchemaName. MySQL has no
// schemas; we treat the connected database as the schema, so there is no
// meaningful static default.
func (d *Dialect) DefaultSchemaName() string { return "" }

// GetCurrentSchema returns the currently selected database, which we treat as
// the active schema.
func (d *Dialect) GetCurrentSchema(ctx context.Context, db *sql.DB) (string, error) {
	var current sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&current); err != nil {
		return "", fmt.Errorf("failed to query current database: %w", err)
	}
	if !current.Valid {
		return "", nil
	}
	return current.String, nil
}

// GetSchemas returns all user databases, excluding the system ones.
func (d *Dialect) GetSchemas(ctx context.Context, db *sql.DB) ([]string, error) {
	const query = `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')
		ORDER BY schema_name ASC;
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query databases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan database row: %w", err)
		}
		schemas = append(schemas, name)
	}
	return schemas, rows.Err()
}

// GetTables returns all base tables in the given database with their columns,
// primary keys, foreign keys, and DDL.
func (d *Dialect) GetTables(ctx context.Context, db *sql.DB, schema string) ([]core.Table, error) {
	const listQuery = `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ?
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name ASC;
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

	columnsByTable, err := d.batchColumns(ctx, db, schema, tableNames)
	if err != nil {
		return nil, err
	}
	pksByTable, err := d.batchPrimaryKeys(ctx, db, schema, tableNames)
	if err != nil {
		return nil, err
	}
	fksByTable, err := d.batchForeignKeys(ctx, db, schema, tableNames)
	if err != nil {
		return nil, err
	}
	ddlByTable, err := d.batchTableDDL(ctx, db, schema, tableNames)
	if err != nil {
		return nil, err
	}

	tables := make([]core.Table, 0, len(tableNames))
	for _, name := range tableNames {
		cols := columnsByTable[name]
		pk := pksByTable[name]
		fk := fksByTable[name]
		core.EnrichColumnsWithConstraints(&cols, pk, fk)
		tables = append(tables, core.Table{
			Name:       name,
			Columns:    cols,
			PrimaryKey: pk,
			DDL:        ddlByTable[name],
		})
	}
	return tables, nil
}

// batchColumns reads columns for all tables in schema in one round-trip.
func (d *Dialect) batchColumns(ctx context.Context, db *sql.DB, schema string, tableNames []string) (map[string][]core.Column, error) {
	placeholders, args := buildInClause(tableNames, schema)
	query := fmt.Sprintf(`
		SELECT table_name, column_name, column_type, is_nullable, column_default, extra
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name IN (%s)
		ORDER BY table_name, ordinal_position;
	`, placeholders)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string][]core.Column)
	for rows.Next() {
		var tableName, colName, colType, isNullable string
		var defaultVal sql.NullString
		var extra sql.NullString
		if err := rows.Scan(&tableName, &colName, &colType, &isNullable, &defaultVal, &extra); err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}
		col := core.Column{
			Name:     colName,
			Type:     colType,
			Nullable: strings.EqualFold(isNullable, "YES"),
		}
		if defaultVal.Valid {
			s := defaultVal.String
			col.Default = &s
		}
		if extra.Valid && extra.String != "" {
			col.Extra = map[string]any{"extra": extra.String}
		}
		out[tableName] = append(out[tableName], col)
	}
	return out, rows.Err()
}

// batchPrimaryKeys reads PRIMARY KEY columns for all tables in schema.
func (d *Dialect) batchPrimaryKeys(ctx context.Context, db *sql.DB, schema string, tableNames []string) (map[string][]string, error) {
	placeholders, args := buildInClause(tableNames, schema)
	query := fmt.Sprintf(`
		SELECT table_name, column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = ? AND table_name IN (%s) AND constraint_name = 'PRIMARY'
		ORDER BY table_name, ordinal_position;
	`, placeholders)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query primary keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string][]string)
	for rows.Next() {
		var tableName, colName string
		if err := rows.Scan(&tableName, &colName); err != nil {
			return nil, fmt.Errorf("failed to scan primary key row: %w", err)
		}
		out[tableName] = append(out[tableName], colName)
	}
	return out, rows.Err()
}

// batchForeignKeys reads foreign-key references for all tables in schema.
func (d *Dialect) batchForeignKeys(ctx context.Context, db *sql.DB, schema string, tableNames []string) (map[string]map[string]core.ForeignKeyRef, error) {
	placeholders, args := buildInClause(tableNames, schema)
	query := fmt.Sprintf(`
		SELECT table_name, column_name, referenced_table_schema, referenced_table_name, referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = ?
		  AND table_name IN (%s)
		  AND referenced_table_name IS NOT NULL;
	`, placeholders)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]map[string]core.ForeignKeyRef)
	for rows.Next() {
		var tableName, colName, refSchema, refTable, refColumn string
		if err := rows.Scan(&tableName, &colName, &refSchema, &refTable, &refColumn); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key row: %w", err)
		}
		if _, ok := out[tableName]; !ok {
			out[tableName] = make(map[string]core.ForeignKeyRef)
		}
		out[tableName][colName] = core.ForeignKeyRef{
			SchemaName: refSchema,
			TableName:  refTable,
			ColumnName: refColumn,
		}
	}
	return out, rows.Err()
}

// batchTableDDL collects SHOW CREATE TABLE output for every table. One query per
// table, but the alternative (joining information_schema views) is incomplete.
func (d *Dialect) batchTableDDL(ctx context.Context, db *sql.DB, schema string, tableNames []string) (map[string]string, error) {
	out := make(map[string]string, len(tableNames))
	for _, name := range tableNames {
		query := fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", escapeBackticks(schema), escapeBackticks(name))
		row := db.QueryRowContext(ctx, query)
		var tname, ddl string
		if err := row.Scan(&tname, &ddl); err != nil {
			continue
		}
		out[name] = ddl
	}
	return out, nil
}

// GetViews returns all views in the given database with their columns and DDL.
func (d *Dialect) GetViews(ctx context.Context, db *sql.DB, schema string) ([]core.Table, error) {
	const listQuery = `
		SELECT table_name
		FROM information_schema.views
		WHERE table_schema = ?
		ORDER BY table_name ASC;
	`
	rows, err := db.QueryContext(ctx, listQuery, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query views: %w", err)
	}
	var viewNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan view row: %w", err)
		}
		viewNames = append(viewNames, name)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(viewNames) == 0 {
		return nil, nil
	}

	columnsByView, err := d.batchColumns(ctx, db, schema, viewNames)
	if err != nil {
		return nil, err
	}

	views := make([]core.Table, 0, len(viewNames))
	for _, name := range viewNames {
		var ddl string
		row := db.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE VIEW `%s`.`%s`", escapeBackticks(schema), escapeBackticks(name)))
		var vname, viewDDL string
		var charset, collation sql.NullString
		if err := row.Scan(&vname, &viewDDL, &charset, &collation); err == nil {
			ddl = viewDDL
		}
		views = append(views, core.Table{
			Name:    name,
			Columns: columnsByView[name],
			DDL:     ddl,
		})
	}
	return views, nil
}

// GetIndexes returns all indexes in the given database, grouped by index name.
func (d *Dialect) GetIndexes(ctx context.Context, db *sql.DB, schema string) ([]core.IndexInfo, error) {
	const query = `
		SELECT table_name, index_name, seq_in_index, column_name, collation
		FROM information_schema.statistics
		WHERE table_schema = ?
		ORDER BY table_name, index_name, seq_in_index;
	`
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type indexKey struct{ table, name string }
	indexMap := make(map[indexKey]*core.IndexInfo)
	var order []indexKey
	for rows.Next() {
		var tableName, indexName, columnName string
		var seqInIndex int
		var collation sql.NullString
		if err := rows.Scan(&tableName, &indexName, &seqInIndex, &columnName, &collation); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}
		key := indexKey{tableName, indexName}
		idx, ok := indexMap[key]
		if !ok {
			idx = &core.IndexInfo{Name: indexName, TableName: tableName}
			indexMap[key] = idx
			order = append(order, key)
		}
		idx.Columns = append(idx.Columns, core.IndexColumnInfo{
			Name:       columnName,
			Position:   seqInIndex,
			Collation:  nullStr(collation),
			Descending: collation.Valid && collation.String == "D",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]core.IndexInfo, 0, len(order))
	for _, k := range order {
		out = append(out, *indexMap[k])
	}
	return out, nil
}

// GetTriggers returns all triggers in the given database.
func (d *Dialect) GetTriggers(ctx context.Context, db *sql.DB, schema string) ([]core.TriggerInfo, error) {
	const query = `
		SELECT trigger_name, event_object_table, action_statement, action_timing, event_manipulation
		FROM information_schema.triggers
		WHERE trigger_schema = ?
		ORDER BY trigger_name ASC;
	`
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query triggers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var triggers []core.TriggerInfo
	for rows.Next() {
		var name, tableName, action, timing, event string
		if err := rows.Scan(&name, &tableName, &action, &timing, &event); err != nil {
			return nil, fmt.Errorf("failed to scan trigger row: %w", err)
		}
		ddl := fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW %s", name, timing, event, tableName, action)
		triggers = append(triggers, core.TriggerInfo{
			Name:      name,
			TableName: tableName,
			DDL:       ddl,
		})
	}
	return triggers, rows.Err()
}

// GetStats returns approximate row counts and data sizes per table.
func (d *Dialect) GetStats(ctx context.Context, db *sql.DB, schema string) (core.TableStats, error) {
	const query = `
		SELECT table_name, table_rows, data_length, index_length
		FROM information_schema.tables
		WHERE table_schema = ?;
	`
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	stats := make(core.TableStats)
	for rows.Next() {
		var name string
		var tableRows, dataLen, indexLen sql.NullInt64
		if err := rows.Scan(&name, &tableRows, &dataLen, &indexLen); err != nil {
			return nil, fmt.Errorf("failed to scan stats row: %w", err)
		}
		stats[name] = fmt.Sprintf("rows=%d data_length=%d index_length=%d", tableRows.Int64, dataLen.Int64, indexLen.Int64)
	}
	return stats, rows.Err()
}

// GetTypes returns one synthetic Type per ENUM or SET column in schema, so
// the editor's Types tree exposes them as named entries. MySQL has no
// CREATE TYPE, so these are derived from information_schema.columns instead
// of a system catalog. Name is "<table>_<column>" to avoid collisions when
// two tables share a column name with different value sets.
func (d *Dialect) GetTypes(ctx context.Context, db *sql.DB, schema string) ([]core.Type, error) {
	const query = `
		SELECT table_name, column_name, data_type, column_type
		FROM information_schema.columns
		WHERE table_schema = ? AND data_type IN ('enum','set')
		ORDER BY table_name, column_name;
	`
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query enum/set columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.Type
	for rows.Next() {
		var table, column, dataType, columnType string
		if err := rows.Scan(&table, &column, &dataType, &columnType); err != nil {
			return nil, fmt.Errorf("failed to scan enum/set row: %w", err)
		}
		// "e" matches the convention EnrichEnumValues uses for named-enum
		// fallback; SET has no Postgres analogue so it gets its own kind.
		kind := "e"
		if dataType == "set" {
			kind = "set"
		}
		out = append(out, core.Type{
			Schema:     schema,
			Name:       table + "_" + column,
			Kind:       kind,
			Display:    columnType,
			EnumLabels: core.ParseInlineEnumValues(columnType),
		})
	}
	return out, rows.Err()
}

// GetFunctions returns stored functions and procedures from the given database.
// Args come from information_schema.parameters, joined in Go to avoid
// GROUP_CONCAT truncation on long signatures. Result uses dtd_identifier so
// length/precision are preserved (e.g. varchar(50) not varchar). Description
// is the routine's COMMENT clause, not its body.
func (d *Dialect) GetFunctions(ctx context.Context, db *sql.DB, schema string) ([]core.Function, error) {
	const routinesQuery = `
		SELECT routine_name, routine_type, COALESCE(dtd_identifier, ''), COALESCE(routine_comment, '')
		FROM information_schema.routines
		WHERE routine_schema = ?
		ORDER BY routine_name ASC;
	`
	rows, err := db.QueryContext(ctx, routinesQuery, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query routines: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fns []core.Function
	for rows.Next() {
		var name, kind, result, comment string
		if err := rows.Scan(&name, &kind, &result, &comment); err != nil {
			return nil, fmt.Errorf("failed to scan routine row: %w", err)
		}
		fns = append(fns, core.Function{
			Schema:      schema,
			Name:        name,
			Kind:        strings.ToLower(kind),
			Result:      result,
			Description: comment,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(fns) == 0 {
		return nil, nil
	}

	argsByRoutine, err := d.batchRoutineArgs(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	for i := range fns {
		fns[i].Args = argsByRoutine[fns[i].Name]
	}
	return fns, nil
}

// batchRoutineArgs reads parameter rows for every routine in the schema and
// builds a postgres-style "mode name type, ..." signature per routine. The
// ordinal_position = 0 row (function return value) is skipped; Result already
// carries that information.
func (d *Dialect) batchRoutineArgs(ctx context.Context, db *sql.DB, schema string) (map[string]string, error) {
	const query = `
		SELECT specific_name, parameter_mode, COALESCE(parameter_name, ''), dtd_identifier
		FROM information_schema.parameters
		WHERE specific_schema = ? AND ordinal_position > 0
		ORDER BY specific_name, ordinal_position;
	`
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query routine parameters: %w", err)
	}
	defer func() { _ = rows.Close() }()

	parts := make(map[string][]string)
	for rows.Next() {
		var routine, mode, name, dtype string
		if err := rows.Scan(&routine, &mode, &name, &dtype); err != nil {
			return nil, fmt.Errorf("failed to scan parameter row: %w", err)
		}
		parts[routine] = append(parts[routine], formatParam(mode, name, dtype))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make(map[string]string, len(parts))
	for routine, ps := range parts {
		out[routine] = strings.Join(ps, ", ")
	}
	return out, nil
}

func formatParam(mode, name, dtype string) string {
	var b strings.Builder
	if mode != "" {
		b.WriteString(strings.ToLower(mode))
		b.WriteByte(' ')
	}
	if name != "" {
		b.WriteString(name)
		b.WriteByte(' ')
	}
	b.WriteString(dtype)
	return b.String()
}

// GetCatalogSchema returns the synthetic mysql_builtin schema containing
// MySQL's built-in types and functions, so completion and the Types tree
// can surface them without per-connection introspection. Functions are
// enriched from mysql.help_topic when available; on any error we fall back
// to the static name-only list.
func (d *Dialect) GetCatalogSchema(ctx context.Context, db *sql.DB) (*core.Schema, error) {
	return &core.Schema{
		Name:      mysqlBuiltinSchema,
		Types:     mysqlBuiltinTypes,
		Functions: enrichBuiltinFunctionsFromHelp(ctx, db, mysqlBuiltinFunctions),
	}, nil
}

// GetSettings returns MySQL system variables with their current values.
// Descriptions are not available from performance_schema.
func (d *Dialect) GetSettings(ctx context.Context, db *sql.DB) ([]core.Setting, error) {
	const query = `
		SELECT variable_name, variable_value
		FROM performance_schema.global_variables
		ORDER BY variable_name;
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query system variables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []core.Setting
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("failed to scan system variable row: %w", err)
		}
		out = append(out, core.Setting{Name: name, Value: value})
	}
	return out, rows.Err()
}

// buildInClause returns "?, ?, ?, ..." and the prefixed args slice
// ([schema, name1, name2, ...]) for IN(...) queries scoped to a schema.
func buildInClause(names []string, schema string) (string, []any) {
	placeholders := make([]string, len(names))
	args := make([]any, 0, len(names)+1)
	args = append(args, schema)
	for i, n := range names {
		placeholders[i] = "?"
		args = append(args, n)
	}
	return strings.Join(placeholders, ", "), args
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
