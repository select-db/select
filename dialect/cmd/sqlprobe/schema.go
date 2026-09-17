package main

import (
	"fmt"
	"strings"

	"github.com/selectDb/dialect/core"
)

// parseSchemaDSL builds Metadata from the compact probe notation, one relation
// per line:
//
//	users(id:int pk, email:text, status:enum(active,banned), org_id:int -> orgs.id)
//	view active_users(id:int, email:text)
//	sales.orders(id:int pk, total:numeric(10,2))
//	func lower(text) -> text
//
// Relations with no schema prefix land in defaultSchema.
func parseSchemaDSL(dsl, defaultSchema string) (core.Metadata, error) {
	meta := core.Metadata{DefaultSchema: defaultSchema}
	schemas := map[string]*core.Schema{}

	schemaFor := func(name string) *core.Schema {
		if s, ok := schemas[name]; ok {
			return s
		}
		s := &core.Schema{Name: name}
		schemas[name] = s
		return s
	}
	// The default schema always exists, so a probe against an empty catalog
	// still resolves unqualified names instead of reporting no schema at all.
	schemaFor(defaultSchema)

	for lineNo, raw := range strings.Split(dsl, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "--") {
			continue
		}

		if rest, ok := cutPrefixWord(line, "func"); ok {
			schemaName, fn, err := parseFunctionDSL(rest, defaultSchema)
			if err != nil {
				return meta, fmt.Errorf("line %d: %w", lineNo+1, err)
			}
			s := schemaFor(schemaName)
			s.Functions = append(s.Functions, fn)
			continue
		}

		kind := "table"
		if rest, ok := cutPrefixWord(line, "view"); ok {
			kind, line = "view", rest
		} else if rest, ok := cutPrefixWord(line, "matview"); ok {
			kind, line = "matview", rest
		}

		open := strings.Index(line, "(")
		if open == -1 || !strings.HasSuffix(line, ")") {
			return meta, fmt.Errorf("line %d: want `name(col:type, ...)`, got %q", lineNo+1, raw)
		}

		schemaName, tableName := splitQualified(strings.TrimSpace(line[:open]), defaultSchema)
		columns, err := parseColumnsDSL(line[open+1:len(line)-1], defaultSchema)
		if err != nil {
			return meta, fmt.Errorf("line %d: %w", lineNo+1, err)
		}

		table := core.Table{Name: tableName, Columns: columns}
		for _, c := range columns {
			if c.IsPrimaryKey {
				table.PrimaryKey = append(table.PrimaryKey, c.Name)
			}
		}

		s := schemaFor(schemaName)
		switch kind {
		case "view":
			s.Views = append(s.Views, table)
		case "matview":
			s.MaterializedViews = append(s.MaterializedViews, table)
		default:
			s.Tables = append(s.Tables, table)
		}

		// An enum column implies the named type exists, so completion and the
		// enum lint rule see the same catalog a real introspection would build.
		for _, c := range columns {
			if len(c.EnumValues) == 0 {
				continue
			}
			s.Types = append(s.Types, core.Type{
				Schema:     schemaName,
				Name:       c.Type,
				Kind:       "enum",
				Display:    c.Type,
				EnumLabels: c.EnumValues,
			})
		}
	}

	for _, s := range schemas {
		meta.Schemas = append(meta.Schemas, *s)
	}
	return meta, nil
}

func parseColumnsDSL(body, defaultSchema string) ([]core.Column, error) {
	var columns []core.Column
	for _, part := range splitTopLevel(body, ',') {
		spec := strings.TrimSpace(part)
		if spec == "" {
			continue
		}

		var fk *core.ForeignKeyRef
		if idx := strings.Index(spec, "->"); idx != -1 {
			ref, err := parseFKRef(strings.TrimSpace(spec[idx+2:]), defaultSchema)
			if err != nil {
				return nil, err
			}
			fk = ref
			spec = strings.TrimSpace(spec[:idx])
		}

		fields := strings.Fields(spec)
		if len(fields) == 0 {
			continue
		}

		col := core.Column{Nullable: true}
		for _, mod := range fields[1:] {
			switch strings.ToLower(mod) {
			case "pk":
				col.IsPrimaryKey = true
				col.Nullable = false
			case "notnull":
				col.Nullable = false
			default:
				return nil, fmt.Errorf("unknown column modifier %q", mod)
			}
		}

		name, typeName, found := strings.Cut(fields[0], ":")
		if !found {
			return nil, fmt.Errorf("want `name:type`, got %q", fields[0])
		}
		col.Name = strings.TrimSpace(name)
		typeName = strings.TrimSpace(typeName)

		if inner, ok := cutCall(typeName, "enum"); ok {
			for _, v := range splitTopLevel(inner, ',') {
				if v = strings.TrimSpace(v); v != "" {
					col.EnumValues = append(col.EnumValues, v)
				}
			}
			// Name the type after the column so two enum columns in one probe
			// stay distinguishable in the catalog.
			typeName = col.Name + "_enum"
		}
		col.Type = typeName

		if fk != nil {
			col.IsForeignKey = true
			col.ForeignKey = fk
		}
		columns = append(columns, col)
	}
	return columns, nil
}

func parseFunctionDSL(rest, defaultSchema string) (string, core.Function, error) {
	signature, result, _ := strings.Cut(rest, "->")
	signature = strings.TrimSpace(signature)

	open := strings.Index(signature, "(")
	if open == -1 || !strings.HasSuffix(signature, ")") {
		return "", core.Function{}, fmt.Errorf("want `func name(args) -> result`, got %q", rest)
	}

	schemaName, fnName := splitQualified(strings.TrimSpace(signature[:open]), defaultSchema)
	return schemaName, core.Function{
		Schema: schemaName,
		Name:   fnName,
		Args:   strings.TrimSpace(signature[open+1 : len(signature)-1]),
		Result: strings.TrimSpace(result),
		Kind:   "function",
	}, nil
}

func parseFKRef(ref, defaultSchema string) (*core.ForeignKeyRef, error) {
	parts := strings.Split(ref, ".")
	switch len(parts) {
	case 2:
		return &core.ForeignKeyRef{SchemaName: defaultSchema, TableName: parts[0], ColumnName: parts[1]}, nil
	case 3:
		return &core.ForeignKeyRef{SchemaName: parts[0], TableName: parts[1], ColumnName: parts[2]}, nil
	default:
		return nil, fmt.Errorf("want `table.column` or `schema.table.column`, got %q", ref)
	}
}

func splitQualified(name, defaultSchema string) (schema, object string) {
	if s, o, found := strings.Cut(name, "."); found {
		return s, o
	}
	return defaultSchema, name
}

// splitTopLevel splits on sep, ignoring separators nested inside parentheses so
// that `numeric(10,2)` and `enum(a,b)` survive a comma split.
func splitTopLevel(s string, sep rune) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + len(string(sep))
			}
		}
	}
	return append(parts, s[start:])
}

func cutPrefixWord(s, word string) (string, bool) {
	if rest, ok := strings.CutPrefix(s, word+" "); ok {
		return strings.TrimSpace(rest), true
	}
	return s, false
}

// cutCall returns the argument text of `name(...)`, matched case-insensitively.
func cutCall(s, name string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(s))
	if !strings.HasPrefix(lower, name+"(") || !strings.HasSuffix(lower, ")") {
		return "", false
	}
	trimmed := strings.TrimSpace(s)
	return trimmed[len(name)+1 : len(trimmed)-1], true
}
