package core

import "strings"

// EnrichEnumValues resolves Column.EnumValues for every column in meta, in
// place. It is the single join from a column's declared type to its allowed
// values, run once when metadata is assembled and consumed by both the cell
// editor and the enum lint rule.
//
// Two shapes are covered:
//
//   - Inline declarations: MySQL emits the full definition as the column
//     type, e.g. enum('a','b') or set('x','y'). Values are parsed directly.
//   - Named types: PostgreSQL emits the type name (optionally schema
//     qualified). The label list is looked up among introspected types
//     whose Kind is enum ("e").
func EnrichEnumValues(meta *Metadata) {
	if meta == nil {
		return
	}
	for si := range meta.Schemas {
		s := &meta.Schemas[si]
		enrichTables(meta, s.Tables)
		enrichTables(meta, s.Views)
		enrichTables(meta, s.MaterializedViews)
		enrichTables(meta, s.ForeignTables)
	}
}

func enrichTables(meta *Metadata, tables []Table) {
	for ti := range tables {
		cols := tables[ti].Columns
		for ci := range cols {
			cols[ci].EnumValues = resolveEnumValues(meta, cols[ci].Type)
		}
	}
}

func resolveEnumValues(meta *Metadata, columnType string) []string {
	if vals := ParseInlineEnumValues(columnType); len(vals) > 0 {
		return vals
	}

	name := columnType
	if idx := strings.LastIndexByte(name, '.'); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	for _, t := range meta.AllTypes() {
		if t.Kind != "e" || len(t.EnumLabels) == 0 {
			continue
		}
		if strings.EqualFold(t.Name, name) ||
			strings.EqualFold(t.Display, name) ||
			strings.EqualFold(t.Display, columnType) {
			return append([]string(nil), t.EnumLabels...)
		}
	}
	return nil
}

// ParseInlineEnumValues extracts the quoted members of an inline enum/set
// declaration such as enum('a','b','c') or set('x','y'). A single quote
// inside a value is escaped MySQL-style by doubling it. Returns nil for
// any other type.
func ParseInlineEnumValues(typeStr string) []string {
	s := strings.TrimSpace(typeStr)
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "enum(") && !strings.HasPrefix(lower, "set(") {
		return nil
	}

	open := strings.IndexByte(s, '(')
	closeIdx := strings.LastIndexByte(s, ')')
	if open < 0 || closeIdx <= open {
		return nil
	}
	inner := s[open+1 : closeIdx]

	var out []string
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if !inQuote {
			if c == '\'' {
				inQuote = true
			}
			continue
		}
		if c == '\'' {
			if i+1 < len(inner) && inner[i+1] == '\'' {
				b.WriteByte('\'')
				i++
				continue
			}
			inQuote = false
			out = append(out, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	return out
}
