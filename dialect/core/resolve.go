package core

// Resolution is the half of inspecting a statement that has nothing to do with
// the grammar: given the relations a clause named and the metadata, say which
// tables and columns the statement reads. Each dialect turns its own parse tree
// into RelationRefs and raw names; from there the rules are the same, and
// keeping one copy of them is what stops the three from drifting apart.

// VirtualNames returns the normalized names that resolve to something the
// statement declared rather than a table: a CTE, or a subquery given an alias.
func VirtualNames(ctes []RelationRef, subqueryColumns map[string][]Column, dialect SQLDialect) map[string]bool {
	names := make(map[string]bool, len(ctes)+len(subqueryColumns))
	for _, cte := range ctes {
		names[dialect.NormalizeIdentifier(cte.Table)] = true
	}
	for name := range subqueryColumns {
		names[dialect.NormalizeIdentifier(name)] = true
	}
	return names
}

// ConvertRelationRefs turns the relations a statement named into the tables a
// permission check reads, dropping the ones that name something the statement
// declared and deduplicating the rest.
//
// A table the metadata does not know resolves to no schema, which is refused
// for every role. Only an unqualified name can be virtual: dropping a qualified
// one would be a read nobody checks.
func ConvertRelationRefs(refs []RelationRef, virtual map[string]bool, meta Metadata, dialect SQLDialect) []InspectTable {
	var tables []InspectTable
	seen := make(map[string]bool)

	for _, ref := range refs {
		if ref.Schema == "" && ref.Table == "" {
			continue
		}
		if !ref.Qualified && virtual[dialect.NormalizeIdentifier(ref.Table)] {
			continue
		}

		schema := ref.Schema
		if !TableExistsInMetadata(meta, schema, ref.Table, dialect) {
			schema = ""
		}

		key := schema + "." + ref.Table
		if seen[key] {
			continue
		}
		seen[key] = true

		table := InspectTable{
			Name:      ref.Table,
			Schema:    schema,
			StartLine: ref.Line,
			StartCol:  ref.Col,
			EndCol:    ref.EndCol,
		}
		if ref.Alias != "" {
			table.Alias = &ref.Alias
		}
		tables = append(tables, table)
	}

	return tables
}

// ResolveCTEColumn finds where a column a CTE returns was read from. The CTE's
// own inspection says so where we have it; otherwise the name is looked up
// across the metadata, which is a guess, but a guess that names a table beats
// one that names none.
func ResolveCTEColumn(columnName string, cte *InspectStatement, meta Metadata, dialect SQLDialect) *InspectField {
	normalized := dialect.NormalizeIdentifier(columnName)

	if cte != nil {
		for _, field := range cte.Fields {
			if dialect.NormalizeIdentifier(field.Name) == normalized {
				return &InspectField{Name: field.Name, Table: field.Table, Schema: field.Schema}
			}
		}
	}

	for _, schema := range meta.Schemas {
		for _, table := range schema.Tables {
			for _, col := range table.Columns {
				if dialect.NormalizeIdentifier(col.Name) == normalized {
					return &InspectField{Name: columnName, Table: table.Name, Schema: schema.Name}
				}
			}
		}
	}

	return nil
}

// ExpandStar returns the fields a star selects from the relations in scope. A
// CTE or an aliased subquery answers for itself; anything else is a table, and
// its columns come from the metadata.
//
// The first relation a name matches answers for it. A CTE shadows a table
// spelled the same way, so reading both would report a read of a table the
// statement never touched.
func ExpandStar(
	refs []RelationRef,
	ctes []RelationRef,
	subqueryColumns map[string][]Column,
	cteResults map[string]*InspectStatement,
	meta Metadata,
	dialect SQLDialect,
) []InspectField {
	var fields []InspectField

	for _, ref := range refs {
		if cte, ok := matchCTE(ctes, ref.Table, dialect); ok {
			fields = append(fields, cteFields(cte, cteResults, meta, dialect)...)
			continue
		}

		if ref.Schema == "" {
			if columns, ok := subqueryColumns[ref.Table]; ok {
				for _, col := range columns {
					fields = append(fields, InspectField{
						Name:   col.Name,
						Table:  ref.Table,
						Schema: meta.DefaultSchema,
					})
				}
				continue
			}
		}

		fields = append(fields, TableFields(meta, ref.Schema, ref.Table, dialect)...)
	}

	return fields
}

// matchCTE returns the CTE a name refers to, if any.
func matchCTE(ctes []RelationRef, name string, dialect SQLDialect) (RelationRef, bool) {
	normalized := dialect.NormalizeIdentifier(name)
	for _, cte := range ctes {
		if dialect.NormalizeIdentifier(cte.Table) == normalized {
			return cte, true
		}
	}
	return RelationRef{}, false
}

// cteFields returns what a CTE selects: its own inspection where we have it,
// and otherwise each declared column resolved against the metadata.
func cteFields(cte RelationRef, cteResults map[string]*InspectStatement, meta Metadata, dialect SQLDialect) []InspectField {
	if result, ok := cteResults[dialect.NormalizeIdentifier(cte.Table)]; ok {
		return result.Fields
	}
	var fields []InspectField
	for _, col := range cte.Columns {
		if resolved := ResolveCTEColumn(col.Name, nil, meta, dialect); resolved != nil {
			fields = append(fields, *resolved)
		}
	}
	return fields
}

// ExpandQualifiedStar returns the fields "t.*" selects, where t is a table name
// or an alias in scope. A name that matches no relation returns none.
func ExpandQualifiedStar(
	prefix string,
	refs []RelationRef,
	ctes []RelationRef,
	cteResults map[string]*InspectStatement,
	meta Metadata,
	dialect SQLDialect,
) []InspectField {
	normalized := dialect.NormalizeIdentifier(prefix)

	for _, ref := range refs {
		name := ref.Alias
		if name == "" {
			name = ref.Table
		}
		if dialect.NormalizeIdentifier(name) != normalized {
			continue
		}
		if cte, ok := matchCTE(ctes, ref.Table, dialect); ok {
			return cteFields(cte, cteResults, meta, dialect)
		}
		return TableFields(meta, ref.Schema, ref.Table, dialect)
	}

	return nil
}
