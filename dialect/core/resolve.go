package core

// Resolution is the half of inspecting a statement that has nothing to do with
// the grammar: given the relations a clause named, say which tables and columns
// the statement reads. Each dialect turns its own parse tree into RelationRefs
// and raw names; from there the rules are the same, and keeping one copy of
// them is what stops the three drifting apart.

// Scope is what a statement declared that a name in it can resolve to instead
// of a table: its CTEs, the columns of each aliased subquery, and the
// inspection of each CTE body where there is one. The zero value is a statement
// that declared nothing.
type Scope struct {
	CTEs       []RelationRef
	Subqueries map[string][]Column
	CTEResults map[string]*InspectStatement
}

// Resolver answers what a statement reads. It holds the metadata the names
// resolve against and the dialect that says how two names compare.
type Resolver struct {
	Meta    Metadata
	Dialect SQLDialect
}

// VirtualNames returns the normalized names in scope that resolve to something
// the statement declared rather than to a table.
func (r Resolver) VirtualNames(s Scope) map[string]bool {
	names := make(map[string]bool, len(s.CTEs)+len(s.Subqueries))
	for _, cte := range s.CTEs {
		names[r.Dialect.NormalizeIdentifier(cte.Table)] = true
	}
	for name := range s.Subqueries {
		names[r.Dialect.NormalizeIdentifier(name)] = true
	}
	return names
}

// Tables turns the relations a statement named into the tables a permission
// check reads, dropping the ones that name something the statement declared and
// deduplicating the rest.
//
// A table the metadata does not know resolves to no schema, which is refused
// for every role. Only an unqualified name can be virtual: dropping a qualified
// one would be a read nobody checks.
func (r Resolver) Tables(refs []RelationRef, s Scope) []InspectTable {
	virtual := r.VirtualNames(s)
	var tables []InspectTable
	seen := make(map[string]bool)

	for _, ref := range refs {
		if ref.Schema == "" && ref.Table == "" {
			continue
		}
		if !ref.Qualified && virtual[r.Dialect.NormalizeIdentifier(ref.Table)] {
			continue
		}

		schema := ref.Schema
		if !TableExistsInMetadata(r.Meta, schema, ref.Table, r.Dialect) {
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

// Star returns the fields a bare star selects from the relations in scope. A
// CTE or an aliased subquery answers for itself; anything else is a table, and
// its columns come from the metadata.
//
// The first relation a name matches answers for it. A CTE shadows a table
// spelled the same way, so reading both would report a read of a table the
// statement never touched.
func (r Resolver) Star(refs []RelationRef, s Scope) []InspectField {
	var fields []InspectField

	for _, ref := range refs {
		if cte, ok := r.matchCTE(s.CTEs, ref.Table); ok {
			fields = append(fields, r.cteFields(cte, s)...)
			continue
		}

		if ref.Schema == "" {
			if columns, ok := s.Subqueries[ref.Table]; ok {
				for _, col := range columns {
					fields = append(fields, InspectField{
						Name:   col.Name,
						Table:  ref.Table,
						Schema: r.Meta.DefaultSchema,
					})
				}
				continue
			}
		}

		fields = append(fields, TableFields(r.Meta, ref.Schema, ref.Table, r.Dialect)...)
	}

	return fields
}

// QualifiedStar returns the fields "t.*" selects, where t is a table name or an
// alias in scope. A name that matches no relation returns none.
func (r Resolver) QualifiedStar(prefix string, refs []RelationRef, s Scope) []InspectField {
	normalized := r.Dialect.NormalizeIdentifier(prefix)

	for _, ref := range refs {
		name := ref.Alias
		if name == "" {
			name = ref.Table
		}
		if r.Dialect.NormalizeIdentifier(name) != normalized {
			continue
		}
		if cte, ok := r.matchCTE(s.CTEs, ref.Table); ok {
			return r.cteFields(cte, s)
		}
		return TableFields(r.Meta, ref.Schema, ref.Table, r.Dialect)
	}

	return nil
}

// CTEColumn finds where a column a CTE returns was read from. The CTE's own
// inspection says so where we have it; otherwise the name is looked up across
// the metadata, which is a guess, but a guess that names a table beats one that
// names none.
func (r Resolver) CTEColumn(columnName string, cte *InspectStatement) *InspectField {
	normalized := r.Dialect.NormalizeIdentifier(columnName)

	if cte != nil {
		for _, field := range cte.Fields {
			if r.Dialect.NormalizeIdentifier(field.Name) == normalized {
				return &InspectField{Name: field.Name, Table: field.Table, Schema: field.Schema}
			}
		}
	}

	for _, schema := range r.Meta.Schemas {
		for _, table := range schema.Tables {
			for _, col := range table.Columns {
				if r.Dialect.NormalizeIdentifier(col.Name) == normalized {
					return &InspectField{Name: columnName, Table: table.Name, Schema: schema.Name}
				}
			}
		}
	}

	return nil
}

// matchCTE returns the CTE a name refers to, if any.
func (r Resolver) matchCTE(ctes []RelationRef, name string) (RelationRef, bool) {
	normalized := r.Dialect.NormalizeIdentifier(name)
	for _, cte := range ctes {
		if r.Dialect.NormalizeIdentifier(cte.Table) == normalized {
			return cte, true
		}
	}
	return RelationRef{}, false
}

// cteFields returns what a CTE selects: its own inspection where we have it,
// and otherwise each declared column resolved against the metadata.
func (r Resolver) cteFields(cte RelationRef, s Scope) []InspectField {
	if result, ok := s.CTEResults[r.Dialect.NormalizeIdentifier(cte.Table)]; ok {
		return result.Fields
	}
	var fields []InspectField
	for _, col := range cte.Columns {
		if resolved := r.CTEColumn(col.Name, nil); resolved != nil {
			fields = append(fields, *resolved)
		}
	}
	return fields
}
