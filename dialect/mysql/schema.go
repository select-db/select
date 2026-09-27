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

// objectKey names a table, view or routine across databases.
type objectKey struct{ schema, name string }

// GetTables returns the base tables of each database with their columns,
// primary keys, foreign keys, and DDL.
func (d *Dialect) GetTables(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.Table, error) {
	out := make(map[string][]core.Table)
	in, args := buildInClause(schemas)
	keys, err := listTables(ctx, db, fmt.Sprintf(`
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema IN (%s)
		  AND table_type = 'BASE TABLE'
		ORDER BY table_schema, table_name;
	`, in), args, "tables")
	if err != nil || len(keys) == 0 {
		return out, err
	}

	columnsByTable, err := d.batchColumns(ctx, db, schemas, "BASE TABLE")
	if err != nil {
		return nil, err
	}
	pksByTable, err := d.batchPrimaryKeys(ctx, db, schemas)
	if err != nil {
		return nil, err
	}
	fksByTable, err := d.batchForeignKeys(ctx, db, schemas)
	if err != nil {
		return nil, err
	}
	ddlByTable, err := d.batchTableDDL(ctx, db, keys)
	if err != nil {
		return nil, err
	}

	for _, k := range keys {
		cols := columnsByTable[k]
		pk := pksByTable[k]
		core.EnrichColumnsWithConstraints(&cols, pk, fksByTable[k])
		out[k.schema] = append(out[k.schema], core.Table{
			Name:       k.name,
			Columns:    cols,
			PrimaryKey: pk,
			DDL:        ddlByTable[k],
		})
	}
	return out, nil
}

// listTables runs a query selecting (schema, name) pairs and closes it before
// returning, so the caller's next query does not hold a second connection.
func listTables(ctx context.Context, db *sql.DB, query string, args []any, what string) ([]objectKey, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s: %w", what, err)
	}
	defer func() { _ = rows.Close() }()

	var keys []objectKey
	for rows.Next() {
		var k objectKey
		if err := rows.Scan(&k.schema, &k.name); err != nil {
			return nil, fmt.Errorf("failed to scan %s row: %w", what, err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// batchColumns reads the columns of every table of tableType in schemas in one
// round trip. The join keeps view columns out of the table load and vice versa.
func (d *Dialect) batchColumns(ctx context.Context, db *sql.DB, schemas []string, tableType string) (map[objectKey][]core.Column, error) {
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT c.table_schema, c.table_name, c.column_name, c.column_type, c.is_nullable, c.column_default, c.extra
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema IN (%s) AND t.table_type = ?
		ORDER BY c.table_schema, c.table_name, c.ordinal_position;
	`, in)
	rows, err := db.QueryContext(ctx, query, append(args, tableType)...)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[objectKey][]core.Column)
	for rows.Next() {
		var k objectKey
		var colName, colType, isNullable string
		var defaultVal sql.NullString
		var extra sql.NullString
		if err := rows.Scan(&k.schema, &k.name, &colName, &colType, &isNullable, &defaultVal, &extra); err != nil {
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
		out[k] = append(out[k], col)
	}
	return out, rows.Err()
}

// batchPrimaryKeys reads PRIMARY KEY columns for all tables in schemas.
func (d *Dialect) batchPrimaryKeys(ctx context.Context, db *sql.DB, schemas []string) (map[objectKey][]string, error) {
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT table_schema, table_name, column_name
		FROM information_schema.key_column_usage
		WHERE table_schema IN (%s) AND constraint_name = 'PRIMARY'
		ORDER BY table_schema, table_name, ordinal_position;
	`, in)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query primary keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[objectKey][]string)
	for rows.Next() {
		var k objectKey
		var colName string
		if err := rows.Scan(&k.schema, &k.name, &colName); err != nil {
			return nil, fmt.Errorf("failed to scan primary key row: %w", err)
		}
		out[k] = append(out[k], colName)
	}
	return out, rows.Err()
}

// batchForeignKeys reads foreign-key references for all tables in schemas.
func (d *Dialect) batchForeignKeys(ctx context.Context, db *sql.DB, schemas []string) (map[objectKey]map[string]core.ForeignKeyRef, error) {
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT table_schema, table_name, column_name, referenced_table_schema, referenced_table_name, referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_schema IN (%s)
		  AND referenced_table_name IS NOT NULL;
	`, in)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[objectKey]map[string]core.ForeignKeyRef)
	for rows.Next() {
		var k objectKey
		var colName, refSchema, refTable, refColumn string
		if err := rows.Scan(&k.schema, &k.name, &colName, &refSchema, &refTable, &refColumn); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key row: %w", err)
		}
		if _, ok := out[k]; !ok {
			out[k] = make(map[string]core.ForeignKeyRef)
		}
		out[k][colName] = core.ForeignKeyRef{
			SchemaName: refSchema,
			TableName:  refTable,
			ColumnName: refColumn,
		}
	}
	return out, rows.Err()
}

// batchTableDDL collects SHOW CREATE TABLE output for every table. One query per
// table: information_schema cannot rebuild the full statement.
func (d *Dialect) batchTableDDL(ctx context.Context, db *sql.DB, keys []objectKey) (map[objectKey]string, error) {
	out := make(map[objectKey]string, len(keys))
	for _, k := range keys {
		query := fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", escapeBackticks(k.schema), escapeBackticks(k.name))
		var tname, ddl string
		if err := db.QueryRowContext(ctx, query).Scan(&tname, &ddl); err != nil {
			continue
		}
		out[k] = ddl
	}
	return out, nil
}

// GetViews returns the views of each database with their columns and DDL.
// DDL stays one SHOW CREATE VIEW per view: information_schema.views has no
// ALGORITHM and always qualifies names, so it cannot reproduce that output.
func (d *Dialect) GetViews(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.Table, error) {
	out := make(map[string][]core.Table)
	in, args := buildInClause(schemas)
	keys, err := listTables(ctx, db, fmt.Sprintf(`
		SELECT table_schema, table_name
		FROM information_schema.views
		WHERE table_schema IN (%s)
		ORDER BY table_schema, table_name;
	`, in), args, "views")
	if err != nil || len(keys) == 0 {
		return out, err
	}

	columnsByView, err := d.batchColumns(ctx, db, schemas, "VIEW")
	if err != nil {
		return nil, err
	}

	for _, k := range keys {
		var ddl string
		row := db.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE VIEW `%s`.`%s`", escapeBackticks(k.schema), escapeBackticks(k.name)))
		var vname, viewDDL string
		var charset, collation sql.NullString
		if err := row.Scan(&vname, &viewDDL, &charset, &collation); err == nil {
			ddl = viewDDL
		}
		out[k.schema] = append(out[k.schema], core.Table{
			Name:    k.name,
			Columns: columnsByView[k],
			DDL:     ddl,
		})
	}
	return out, nil
}

// GetIndexes returns the indexes of each database, grouped by index name.
func (d *Dialect) GetIndexes(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.IndexInfo, error) {
	out := make(map[string][]core.IndexInfo)
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT table_schema, table_name, index_name, seq_in_index, column_name, collation
		FROM information_schema.statistics
		WHERE table_schema IN (%s)
		ORDER BY table_schema, table_name, index_name, seq_in_index;
	`, in)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type indexKey struct{ schema, table, name string }
	indexMap := make(map[indexKey]*core.IndexInfo)
	var order []indexKey
	for rows.Next() {
		var schema, tableName, indexName, columnName string
		var seqInIndex int
		var collation sql.NullString
		if err := rows.Scan(&schema, &tableName, &indexName, &seqInIndex, &columnName, &collation); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}
		key := indexKey{schema, tableName, indexName}
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

	for _, k := range order {
		out[k.schema] = append(out[k.schema], *indexMap[k])
	}
	return out, nil
}

// GetTriggers returns the triggers of each database.
func (d *Dialect) GetTriggers(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.TriggerInfo, error) {
	out := make(map[string][]core.TriggerInfo)
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT trigger_schema, trigger_name, event_object_table, action_statement, action_timing, event_manipulation
		FROM information_schema.triggers
		WHERE trigger_schema IN (%s)
		ORDER BY trigger_schema, trigger_name;
	`, in)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query triggers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var schema, name, tableName, action, timing, event string
		if err := rows.Scan(&schema, &name, &tableName, &action, &timing, &event); err != nil {
			return nil, fmt.Errorf("failed to scan trigger row: %w", err)
		}
		ddl := fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW %s", name, timing, event, tableName, action)
		out[schema] = append(out[schema], core.TriggerInfo{
			Name:      name,
			TableName: tableName,
			DDL:       ddl,
		})
	}
	return out, rows.Err()
}

// GetStats returns approximate row counts and data sizes per table of each database.
func (d *Dialect) GetStats(ctx context.Context, db *sql.DB, schemas []string) (map[string]core.TableStats, error) {
	out := make(map[string]core.TableStats)
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT table_schema, table_name, table_rows, data_length, index_length
		FROM information_schema.tables
		WHERE table_schema IN (%s);
	`, in)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var schema, name string
		var tableRows, dataLen, indexLen sql.NullInt64
		if err := rows.Scan(&schema, &name, &tableRows, &dataLen, &indexLen); err != nil {
			return nil, fmt.Errorf("failed to scan stats row: %w", err)
		}
		stats, ok := out[schema]
		if !ok {
			stats = make(core.TableStats)
			out[schema] = stats
		}
		stats[name] = fmt.Sprintf("rows=%d data_length=%d index_length=%d", tableRows.Int64, dataLen.Int64, indexLen.Int64)
	}
	return out, rows.Err()
}

// GetTypes returns one synthetic Type per ENUM or SET column of each database,
// so the editor's Types tree exposes them as named entries. MySQL has no
// CREATE TYPE, so these are derived from information_schema.columns instead
// of a system catalog. Name is "<table>_<column>" to avoid collisions when
// two tables share a column name with different value sets.
func (d *Dialect) GetTypes(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.Type, error) {
	out := make(map[string][]core.Type)
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT table_schema, table_name, column_name, data_type, column_type
		FROM information_schema.columns
		WHERE table_schema IN (%s) AND data_type IN ('enum','set')
		ORDER BY table_schema, table_name, column_name;
	`, in)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query enum/set columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var schema, table, column, dataType, columnType string
		if err := rows.Scan(&schema, &table, &column, &dataType, &columnType); err != nil {
			return nil, fmt.Errorf("failed to scan enum/set row: %w", err)
		}
		// "e" matches the convention EnrichEnumValues uses for named-enum
		// fallback; SET has no Postgres analogue so it gets its own kind.
		kind := "e"
		if dataType == "set" {
			kind = "set"
		}
		out[schema] = append(out[schema], core.Type{
			Schema:     schema,
			Name:       table + "_" + column,
			Kind:       kind,
			Display:    columnType,
			EnumLabels: core.ParseInlineEnumValues(columnType),
		})
	}
	return out, rows.Err()
}

// GetFunctions returns stored functions and procedures of each database.
// Args come from information_schema.parameters, joined in Go to avoid
// GROUP_CONCAT truncation on long signatures. Result uses dtd_identifier so
// length/precision are preserved (e.g. varchar(50) not varchar). Description
// is the routine's COMMENT clause, not its body.
func (d *Dialect) GetFunctions(ctx context.Context, db *sql.DB, schemas []string) (map[string][]core.Function, error) {
	argsByRoutine, err := d.batchRoutineArgs(ctx, db, schemas)
	if err != nil {
		return nil, err
	}
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT routine_schema, routine_name, routine_type, COALESCE(dtd_identifier, ''), COALESCE(routine_comment, '')
		FROM information_schema.routines
		WHERE routine_schema IN (%s)
		ORDER BY routine_schema, routine_name;
	`, in)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query routines: %w", err)
	}
	defer rows.Close()
	out := make(map[string][]core.Function)
	for rows.Next() {
		var schema, name, kind, result, comment string
		if err := rows.Scan(&schema, &name, &kind, &result, &comment); err != nil {
			return nil, fmt.Errorf("failed to scan routine row: %w", err)
		}
		out[schema] = append(out[schema], core.Function{
			Schema:      schema,
			Name:        name,
			Kind:        strings.ToLower(kind),
			Args:        argsByRoutine[objectKey{schema, name}],
			Result:      result,
			Description: comment,
		})
	}
	return out, rows.Err()
}

// batchRoutineArgs reads parameter rows for every routine in schemas and
// builds a postgres-style "mode name type, ..." signature per routine. The
// ordinal_position = 0 row (function return value) is skipped; Result already
// carries that information.
func (d *Dialect) batchRoutineArgs(ctx context.Context, db *sql.DB, schemas []string) (map[objectKey]string, error) {
	in, args := buildInClause(schemas)
	query := fmt.Sprintf(`
		SELECT specific_schema, specific_name, parameter_mode, COALESCE(parameter_name, ''), dtd_identifier
		FROM information_schema.parameters
		WHERE specific_schema IN (%s) AND ordinal_position > 0
		ORDER BY specific_schema, specific_name, ordinal_position;
	`, in)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query routine parameters: %w", err)
	}
	defer func() { _ = rows.Close() }()

	parts := make(map[objectKey][]string)
	for rows.Next() {
		var k objectKey
		var mode, name, dtype string
		if err := rows.Scan(&k.schema, &k.name, &mode, &name, &dtype); err != nil {
			return nil, fmt.Errorf("failed to scan parameter row: %w", err)
		}
		parts[k] = append(parts[k], formatParam(mode, name, dtype))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make(map[objectKey]string, len(parts))
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

// buildInClause returns "?, ?, ..." with one placeholder per value, and the
// values as query args, so names are never spliced into the SQL text.
func buildInClause(values []string) (string, []any) {
	// IN () is a syntax error; IN (NULL) matches nothing.
	if len(values) == 0 {
		return "NULL", nil
	}
	placeholders := make([]string, len(values))
	args := make([]any, len(values))
	for i, v := range values {
		placeholders[i] = "?"
		args[i] = v
	}
	return strings.Join(placeholders, ", "), args
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
