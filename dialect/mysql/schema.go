package mysql

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	core "github.com/selectDb/dialect/core"
)

const userSchemas = "NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')"

// schemaQuery reads every user database in one round trip; with no arguments
// the driver sends it as text, without a prepare. The mysql_* kinds carry what
// the DDL needs and become core rows in fillSchemaRows.
const schemaQuery = `
	SELECT 'schema' AS kind, JSON_OBJECT('name', schema_name, 'catalog', FALSE) AS doc
	FROM information_schema.schemata
	WHERE schema_name ` + userSchemas + `
	UNION ALL SELECT 'schema', JSON_OBJECT('name', '` + mysqlBuiltinSchema + `', 'catalog', TRUE)
	UNION ALL SELECT 'current', JSON_OBJECT('name', COALESCE(DATABASE(), ''))
	UNION ALL SELECT 'mysql_server', JSON_OBJECT('mariadb', VERSION() LIKE '%MariaDB%')
	UNION ALL
	SELECT 'mysql_table', JSON_OBJECT(
		'schema', table_schema, 'name', table_name,
		'kind', CASE table_type WHEN 'BASE TABLE' THEN 'table' WHEN 'VIEW' THEN 'view' ELSE '' END,
		'engine', COALESCE(engine, ''), 'collation', COALESCE(table_collation, ''),
		'comment', COALESCE(table_comment, ''), 'autoIncrement', COALESCE(auto_increment, 0),
		'rows', COALESCE(table_rows, 0), 'dataLength', COALESCE(data_length, 0),
		'indexLength', COALESCE(index_length, 0))
	FROM information_schema.tables
	WHERE table_schema ` + userSchemas + `
	UNION ALL
	-- On MySQL, which has no views.algorithm, the subquery resolves it to no_algorithm's ''.
	SELECT 'mysql_view', JSON_OBJECT(
		'schema', v.table_schema, 'name', v.table_name, 'definition', v.view_definition,
		'checkOption', v.check_option, 'definer', v.definer, 'security', v.security_type,
		'algorithm', (SELECT algorithm FROM information_schema.views a
			WHERE a.table_schema = v.table_schema AND a.table_name = v.table_name))
	FROM information_schema.views v CROSS JOIN (SELECT '' AS algorithm) no_algorithm
	WHERE v.table_schema ` + userSchemas + `
	UNION ALL
	SELECT 'mysql_column', JSON_OBJECT(
		'schema', table_schema, 'table', table_name, 'name', column_name, 'type', column_type,
		'nullable', is_nullable = 'YES', 'default', column_default, 'position', ordinal_position,
		'dataType', data_type, 'extraText', COALESCE(extra, ''),
		'collation', COALESCE(collation_name, ''), 'charset', COALESCE(character_set_name, ''),
		'comment', COALESCE(column_comment, ''), 'generated', COALESCE(generation_expression, ''))
	FROM information_schema.columns
	WHERE table_schema ` + userSchemas + `
	UNION ALL
	SELECT 'mysql_index_column', JSON_OBJECT(
		'schema', table_schema, 'table', table_name, 'index', index_name,
		'name', COALESCE(column_name, ''), 'position', seq_in_index,
		'collation', COALESCE(collation, ''), 'descending', collation = 'D',
		'unique', non_unique = 0, 'indexType', index_type, 'subPart', sub_part,
		'comment', COALESCE(index_comment, ''))
	FROM information_schema.statistics
	WHERE table_schema ` + userSchemas + `
	UNION ALL
	SELECT 'mysql_foreign_key', JSON_OBJECT(
		'schema', k.table_schema, 'table', k.table_name, 'column', k.column_name,
		'refSchema', k.referenced_table_schema, 'refTable', k.referenced_table_name,
		'refColumn', k.referenced_column_name, 'constraint', k.constraint_name,
		'position', k.ordinal_position,
		'onUpdate', COALESCE(r.update_rule, ''), 'onDelete', COALESCE(r.delete_rule, ''))
	FROM information_schema.key_column_usage k
	LEFT JOIN information_schema.referential_constraints r
		ON r.constraint_schema = k.constraint_schema AND r.constraint_name = k.constraint_name
		AND r.table_name = k.table_name
	WHERE k.table_schema ` + userSchemas + ` AND k.referenced_table_name IS NOT NULL
	UNION ALL
	SELECT 'mysql_trigger', JSON_OBJECT(
		'schema', trigger_schema, 'table', event_object_table, 'name', trigger_name,
		'timing', action_timing, 'event', event_manipulation, 'action', action_statement)
	FROM information_schema.triggers
	WHERE trigger_schema ` + userSchemas + `
	UNION ALL
	SELECT 'function', JSON_OBJECT(
		'schema', routine_schema, 'name', routine_name, 'kind', LOWER(routine_type),
		'result', COALESCE(dtd_identifier, ''), 'description', COALESCE(routine_comment, ''))
	FROM information_schema.routines
	WHERE routine_schema ` + userSchemas + `
	UNION ALL
	SELECT 'mysql_parameter', JSON_OBJECT(
		'schema', specific_schema, 'routine', specific_name, 'kind', LOWER(routine_type),
		'mode', COALESCE(parameter_mode, ''), 'name', COALESCE(parameter_name, ''),
		'type', dtd_identifier, 'position', ordinal_position)
	FROM information_schema.parameters
	WHERE specific_schema ` + userSchemas + ` AND ordinal_position > 0`

// settingsQuery fails when performance_schema is off or unreadable, which
// leaves the schema without settings.
const settingsQuery = `
	SELECT 'setting', JSON_OBJECT('name', variable_name, 'value', variable_value)
	FROM performance_schema.global_variables`

// ReadSchema reads every user database in one query, then the server
// variables and the help corpus, each of which may be unreadable.
func (d *Dialect) ReadSchema(ctx context.Context, db *sql.DB) (*core.Metadata, error) {
	rows, err := core.QuerySchemaRows(ctx, db, schemaQuery)
	if err != nil {
		return nil, err
	}
	if settings, err := core.QuerySchemaRows(ctx, db, settingsQuery); err == nil {
		rows.Settings = settings.Settings
	}
	if err := fillSchemaRows(&rows); err != nil {
		return nil, err
	}
	meta := rows.Metadata()
	catalog := &meta.Schemas[len(meta.Schemas)-1]
	catalog.Types = mysqlBuiltinTypes
	catalog.Functions = enrichBuiltinFunctionsFromHelp(ctx, db, mysqlBuiltinFunctions)
	return meta, nil
}

type tableRow struct {
	core.RelationRow
	Engine        string `json:"engine"`
	Collation     string `json:"collation"`
	Comment       string `json:"comment"`
	AutoIncrement int64  `json:"autoIncrement"`
	Rows          int64  `json:"rows"`
	DataLength    int64  `json:"dataLength"`
	IndexLength   int64  `json:"indexLength"`
}

type viewRow struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Definition  string `json:"definition"`
	CheckOption string `json:"checkOption"`
	Definer     string `json:"definer"`
	Security    string `json:"security"`
	Algorithm   string `json:"algorithm"`
}

type columnRow struct {
	core.ColumnRow
	DataType  string `json:"dataType"`
	ExtraText string `json:"extraText"`
	Collation string `json:"collation"`
	Charset   string `json:"charset"`
	Comment   string `json:"comment"`
	Generated string `json:"generated"`
}

type indexColumnRow struct {
	core.IndexColumnRow
	Unique    core.Flag `json:"unique"`
	IndexType string    `json:"indexType"`
	SubPart   *int      `json:"subPart"`
	Comment   string    `json:"comment"`
}

type foreignKeyRow struct {
	core.ForeignKeyRow
	Constraint string `json:"constraint"`
	Position   int    `json:"position"`
	OnUpdate   string `json:"onUpdate"`
	OnDelete   string `json:"onDelete"`
}

type triggerRow struct {
	core.TriggerRow
	Timing string `json:"timing"`
	Event  string `json:"event"`
	Action string `json:"action"`
}

type parameterRow struct {
	Schema   string `json:"schema"`
	Routine  string `json:"routine"`
	Kind     string `json:"kind"`
	Mode     string `json:"mode"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Position int    `json:"position"`
}

// tableKey names a table or view across databases.
type tableKey struct{ schema, name string }

// fillSchemaRows turns the mysql_* rows into core rows: relations with their
// DDL, statistics, columns, ENUM and SET types, indexes and routine arguments.
func fillSchemaRows(r *core.SchemaRows) error {
	var (
		server []struct {
			MariaDB core.Flag `json:"mariadb"`
		}
		tables       []tableRow
		views        []viewRow
		columns      []columnRow
		indexColumns []indexColumnRow
		foreignKeys  []foreignKeyRow
		triggers     []triggerRow
		parameters   []parameterRow
	)
	for kind, dst := range map[string]any{
		"mysql_server": &server, "mysql_table": &tables, "mysql_view": &views,
		"mysql_column": &columns, "mysql_index_column": &indexColumns,
		"mysql_foreign_key": &foreignKeys, "mysql_trigger": &triggers, "mysql_parameter": &parameters,
	} {
		if err := r.DecodeOther(kind, dst); err != nil {
			return err
		}
	}
	mariadb := len(server) > 0 && bool(server[0].MariaDB)

	slices.SortStableFunc(columns, func(a, b columnRow) int { return cmp.Compare(a.Position, b.Position) })
	columnsByTable := make(map[tableKey][]columnRow)
	for _, c := range columns {
		if c.ExtraText != "" {
			c.Extra = map[string]any{"extra": c.ExtraText}
		}
		r.Columns = append(r.Columns, c.ColumnRow)
		k := tableKey{c.Schema, c.Table}
		columnsByTable[k] = append(columnsByTable[k], c)
		if c.DataType == "enum" || c.DataType == "set" {
			// "e" is the kind EnrichEnumValues reads; SET has no Postgres analogue.
			kind := "e"
			if c.DataType == "set" {
				kind = "set"
			}
			r.Types = append(r.Types, core.Type{
				Schema: c.Schema, Name: c.Table + "_" + c.Name, Kind: kind, Display: c.Type,
				EnumLabels: core.ParseInlineEnumValues(c.Type),
			})
		}
	}

	indexesByTable := make(map[tableKey][]indexColumnRow)
	seenIndex := make(map[[3]string]bool)
	for _, ic := range indexColumns {
		if k := [3]string{ic.Schema, ic.Table, ic.Index}; !seenIndex[k] {
			seenIndex[k] = true
			r.Indexes = append(r.Indexes, core.IndexRow{Schema: ic.Schema, Table: ic.Table, Name: ic.Index})
		}
		r.IndexColumns = append(r.IndexColumns, ic.IndexColumnRow)
		if ic.Index == "PRIMARY" {
			r.PrimaryKeys = append(r.PrimaryKeys, core.KeyColumnRow{
				Schema: ic.Schema, Table: ic.Table, Column: ic.Name, Position: ic.Position,
			})
		}
		k := tableKey{ic.Schema, ic.Table}
		indexesByTable[k] = append(indexesByTable[k], ic)
	}

	foreignKeysByTable := make(map[tableKey][]foreignKeyRow)
	for _, fk := range foreignKeys {
		r.ForeignKeys = append(r.ForeignKeys, fk.ForeignKeyRow)
		k := tableKey{fk.Schema, fk.Table}
		foreignKeysByTable[k] = append(foreignKeysByTable[k], fk)
	}

	for _, t := range triggers {
		t.DDL = fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW %s",
			quoteIdent(t.Name), t.Timing, t.Event, quoteIdent(t.Table), t.Action)
		r.Triggers = append(r.Triggers, t.TriggerRow)
	}

	// Functions and procedures are separate namespaces, so the kind is in the key.
	slices.SortStableFunc(parameters, func(a, b parameterRow) int { return cmp.Compare(a.Position, b.Position) })
	args := make(map[[3]string][]string)
	for _, p := range parameters {
		k := [3]string{p.Schema, p.Kind, p.Routine}
		args[k] = append(args[k], formatParam(p.Mode, p.Name, p.Type))
	}
	for i, f := range r.Functions {
		r.Functions[i].Args = strings.Join(args[[3]string{f.Schema, f.Kind, f.Name}], ", ")
	}

	viewDDL := make(map[tableKey]string, len(views))
	for _, v := range views {
		viewDDL[tableKey{v.Schema, v.Name}] = buildViewDDL(v)
	}
	for _, t := range tables {
		r.Stats = append(r.Stats, core.StatRow{
			Schema: t.Schema, Name: t.Name,
			Value: fmt.Sprintf("rows=%d data_length=%d index_length=%d", t.Rows, t.DataLength, t.IndexLength),
		})
		k := tableKey{t.Schema, t.Name}
		switch t.Kind {
		case "table":
			t.DDL = buildTableDDL(t, columnsByTable[k], indexesByTable[k], foreignKeysByTable[k], mariadb)
		case "view":
			t.DDL = viewDDL[k]
		default:
			continue
		}
		r.Relations = append(r.Relations, t.RelationRow)
	}
	return nil
}

// buildTableDDL approximates SHOW CREATE TABLE from the catalog rows, which
// saves a round trip per table.
func buildTableDDL(t tableRow, columns []columnRow, indexColumns []indexColumnRow, foreignKeys []foreignKeyRow, mariadb bool) string {
	var lines []string
	for _, c := range columns {
		lines = append(lines, "  "+columnDDL(c, t.Collation, mariadb))
	}

	var indexNames []string
	byIndex := make(map[string][]indexColumnRow)
	for _, ic := range indexColumns {
		if byIndex[ic.Index] == nil {
			indexNames = append(indexNames, ic.Index)
		}
		byIndex[ic.Index] = append(byIndex[ic.Index], ic)
	}
	// SHOW CREATE lists the primary key, then unique keys, then the rest.
	rank := func(name string) int {
		switch {
		case name == "PRIMARY":
			return 0
		case bool(byIndex[name][0].Unique):
			return 1
		}
		return 2
	}
	slices.SortStableFunc(indexNames, func(a, b string) int { return cmp.Compare(rank(a), rank(b)) })
	for _, name := range indexNames {
		parts := byIndex[name]
		slices.SortStableFunc(parts, func(a, b indexColumnRow) int { return cmp.Compare(a.Position, b.Position) })
		var cols []string
		for _, p := range parts {
			if p.Name == "" {
				continue
			}
			col := quoteIdent(p.Name)
			// A spatial key reports a sub_part that SHOW CREATE leaves out.
			if p.SubPart != nil && p.IndexType != "SPATIAL" {
				col += fmt.Sprintf("(%d)", *p.SubPart)
			}
			if p.Descending {
				col += " DESC"
			}
			cols = append(cols, col)
		}
		// An expression key part has no column name to write, and "()" is not DDL.
		if len(cols) == 0 {
			continue
		}
		var line string
		switch {
		case name == "PRIMARY":
			line = "PRIMARY KEY"
		case parts[0].IndexType == "FULLTEXT" || parts[0].IndexType == "SPATIAL":
			line = parts[0].IndexType + " KEY " + quoteIdent(name)
		case bool(parts[0].Unique):
			line = "UNIQUE KEY " + quoteIdent(name)
		default:
			line = "KEY " + quoteIdent(name)
		}
		line = "  " + line + " (" + strings.Join(cols, ",") + ")"
		if parts[0].Comment != "" {
			line += " COMMENT " + quoteLiteral(parts[0].Comment)
		}
		lines = append(lines, line)
	}

	var constraints []string
	byConstraint := make(map[string][]foreignKeyRow)
	for _, fk := range foreignKeys {
		if byConstraint[fk.Constraint] == nil {
			constraints = append(constraints, fk.Constraint)
		}
		byConstraint[fk.Constraint] = append(byConstraint[fk.Constraint], fk)
	}
	slices.Sort(constraints)
	sep := ","
	if mariadb {
		sep = ", "
	}
	for _, name := range constraints {
		parts := byConstraint[name]
		slices.SortStableFunc(parts, func(a, b foreignKeyRow) int { return cmp.Compare(a.Position, b.Position) })
		var cols, refCols []string
		for _, p := range parts {
			cols = append(cols, quoteIdent(p.Column))
			refCols = append(refCols, quoteIdent(p.RefColumn))
		}
		ref := quoteIdent(parts[0].RefTable)
		if parts[0].RefSchema != t.Schema {
			ref = quoteIdent(parts[0].RefSchema) + "." + ref
		}
		line := fmt.Sprintf("  CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
			quoteIdent(name), strings.Join(cols, sep), ref, strings.Join(refCols, sep))
		// Both servers leave out the rule a key gets when none is written.
		for _, rule := range [][2]string{{"DELETE", parts[0].OnDelete}, {"UPDATE", parts[0].OnUpdate}} {
			if rule[1] != "" && rule[1] != "RESTRICT" && rule[1] != "NO ACTION" {
				line += " ON " + rule[0] + " " + rule[1]
			}
		}
		lines = append(lines, line)
	}

	ddl := "CREATE TABLE " + quoteIdent(t.Name) + " (\n" + strings.Join(lines, ",\n") + "\n)"
	if t.Engine != "" {
		ddl += " ENGINE=" + t.Engine
	}
	if t.AutoIncrement > 1 {
		ddl += fmt.Sprintf(" AUTO_INCREMENT=%d", t.AutoIncrement)
	}
	if t.Collation != "" {
		// A collation name starts with its character set on both servers.
		charset, _, _ := strings.Cut(t.Collation, "_")
		ddl += " DEFAULT CHARSET=" + charset + " COLLATE=" + t.Collation
	}
	if t.Comment != "" {
		ddl += " COMMENT=" + quoteLiteral(t.Comment)
	}
	return ddl
}

// columnDDL writes one column as SHOW CREATE TABLE does. MariaDB reports
// defaults as SQL text; MySQL reports the bare value unless it is an expression.
func columnDDL(c columnRow, tableCollation string, mariadb bool) string {
	s := quoteIdent(c.Name) + " " + c.Type
	if c.Collation != "" && c.Collation != tableCollation {
		s += " CHARACTER SET " + c.Charset + " COLLATE " + c.Collation
	}
	extra := strings.ToLower(c.ExtraText)
	if c.Generated != "" {
		kind := "VIRTUAL"
		if strings.Contains(extra, "stored") {
			kind = "STORED"
		}
		s += " GENERATED ALWAYS AS (" + c.Generated + ") " + kind
	}
	if !c.Nullable {
		s += " NOT NULL"
	} else if c.DataType == "timestamp" {
		s += " NULL"
	}
	invisible := strings.Contains(extra, "invisible")
	if invisible && mariadb {
		s += " INVISIBLE"
	}
	switch {
	case c.Generated != "": // a generated column has no default
	case c.Default != nil && (mariadb || c.DataType == "bit"):
		s += " DEFAULT " + *c.Default
	// MySQL 5.7 has no DEFAULT_GENERATED, so CURRENT_TIMESTAMP is known by name.
	case c.Default != nil && strings.HasPrefix(strings.ToUpper(*c.Default), "CURRENT_TIMESTAMP"):
		s += " DEFAULT " + *c.Default
	case c.Default != nil && strings.Contains(extra, "default_generated"):
		s += " DEFAULT (" + *c.Default + ")"
	case c.Default != nil:
		s += " DEFAULT " + quoteLiteral(*c.Default)
	case bool(c.Nullable) && !mariadb && !strings.Contains(extra, "auto_increment") && !noDefaultNull(c.DataType):
		s += " DEFAULT NULL"
	}
	if strings.Contains(extra, "auto_increment") {
		s += " AUTO_INCREMENT"
	}
	if i := strings.Index(extra, "on update "); i >= 0 {
		s += " ON UPDATE " + c.ExtraText[i+len("on update "):]
	}
	if invisible && !mariadb {
		s += " /*!80023 INVISIBLE */"
	}
	if c.Comment != "" {
		s += " COMMENT " + quoteLiteral(c.Comment)
	}
	return s
}

// noDefaultNull reports the types MySQL prints without DEFAULT NULL.
func noDefaultNull(dataType string) bool {
	switch dataType {
	case "tinytext", "text", "mediumtext", "longtext", "tinyblob", "blob", "mediumblob", "longblob",
		"json", "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring",
		"multipolygon", "geometrycollection", "geomcollection":
		return true
	}
	return false
}

// buildViewDDL approximates SHOW CREATE VIEW. information_schema qualifies
// names with the view's own database, which SHOW CREATE leaves out.
func buildViewDDL(v viewRow) string {
	// The definition is empty without the SHOW VIEW privilege.
	if v.Definition == "" {
		return ""
	}
	ddl := "CREATE "
	if v.Algorithm != "" {
		ddl += "ALGORITHM=" + v.Algorithm + " "
	}
	// The user name may itself hold an @, the host may not.
	if i := strings.LastIndex(v.Definer, "@"); i >= 0 {
		ddl += "DEFINER=" + quoteIdent(v.Definer[:i]) + "@" + quoteIdent(v.Definer[i+1:]) + " "
	}
	if v.Security != "" {
		ddl += "SQL SECURITY " + v.Security + " "
	}
	ddl += "VIEW " + quoteIdent(v.Name) + " AS " + unqualify(v.Definition, v.Schema)
	if v.CheckOption != "" && v.CheckOption != "NONE" {
		ddl += " WITH " + v.CheckOption + " CHECK OPTION"
	}
	return ddl
}

// unqualify drops `schema`. where it starts a name, leaving a table that
// happens to share the schema's name, as in `other`.`schema`.`col`, alone.
func unqualify(definition, schema string) string {
	prefix := quoteIdent(schema) + "."
	var b strings.Builder
	for {
		i := strings.Index(definition, prefix)
		if i < 0 {
			b.WriteString(definition)
			return b.String()
		}
		b.WriteString(definition[:i])
		if i > 0 && definition[i-1] == '.' {
			b.WriteString(prefix)
		}
		definition = definition[i+len(prefix):]
	}
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
