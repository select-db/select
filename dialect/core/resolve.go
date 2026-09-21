package core

// Resolution is the half of inspecting a statement that has nothing to do with
// the grammar: given the relations a clause named, say which tables and columns
// the statement reads.

// Scope is what a statement declared that a name in it can resolve to instead
// of a table. The zero value is a statement that declared nothing.
//
// Subqueries is keyed by the alias as written; CTEResults by the normalized CTE
// name.
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
// check reads. A table the metadata does not know resolves to no schema, which
// is refused for every role. Only an unqualified name can be virtual: dropping
// a qualified one would be a read nobody checks.
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
		if cte, ok := r.matchCTE(s.CTEs, ref); ok {
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
		if cte, ok := r.matchCTE(s.CTEs, ref); ok {
			return r.cteFields(cte, s)
		}
		return TableFields(r.Meta, ref.Schema, ref.Table, r.Dialect)
	}

	return nil
}

// CTEColumn finds where a column a CTE returns was read from. The CTE's own
// inspection says so where we have it; otherwise the name is looked up across
// the metadata, which names a table where a failure would name none.
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

// DropVirtual strips, throughout stmts, the tables naming something in virtual.
func (r Resolver) DropVirtual(stmts []InspectStatement, virtual map[string]bool) {
	DropVirtualTables(stmts, virtual, r.Dialect.NormalizeIdentifier)
}

// DropCTETables strips the tables naming a CTE the enclosing statement
// declared. Only the CTE names travel: a subquery alias is not a relation
// outside the statement that declared it.
func (r Resolver) DropCTETables(stmts []InspectStatement, ctes []RelationRef) {
	r.DropVirtual(stmts, r.VirtualNames(Scope{CTEs: ctes}))
}

// matchCTE returns the CTE a relation refers to, if any. A qualified name is
// the real table even where a CTE shadows the bare one, which is the rule
// Tables applies to the same relations.
func (r Resolver) matchCTE(ctes []RelationRef, ref RelationRef) (RelationRef, bool) {
	if ref.Qualified {
		return RelationRef{}, false
	}
	normalized := r.Dialect.NormalizeIdentifier(ref.Table)
	for _, cte := range ctes {
		if r.Dialect.NormalizeIdentifier(cte.Table) == normalized {
			return cte, true
		}
	}
	return RelationRef{}, false
}

// cteFields returns what a CTE selects. The inspectors walk a WITH clause one
// body per name, so a CTE in scope has its own inspection; a name with none
// selects nothing rather than guessing a table from the metadata.
func (r Resolver) cteFields(cte RelationRef, s Scope) []InspectField {
	if result, ok := s.CTEResults[r.Dialect.NormalizeIdentifier(cte.Table)]; ok {
		return result.Fields
	}
	return nil
}

// NamedColumns returns the columns the bare names in a USING list refer to, one
// field per relation in scope that carries the name: a join on USING (c) reads
// c on both sides.
func (r Resolver) NamedColumns(names []string, refs []RelationRef, s Scope) []InspectField {
	if len(names) == 0 {
		return nil
	}
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[r.Dialect.NormalizeIdentifier(name)] = true
	}
	var fields []InspectField
	for _, field := range r.Star(refs, s) {
		if wanted[r.Dialect.NormalizeIdentifier(field.Name)] {
			fields = append(fields, field)
		}
	}
	return fields
}

// SharedColumns returns the columns a NATURAL join pairs rows on: every name
// more than one relation in scope carries.
func (r Resolver) SharedColumns(refs []RelationRef, s Scope) []InspectField {
	all := r.Star(refs, s)
	relations := make(map[string]map[string]bool, len(all))
	for _, field := range all {
		name := r.Dialect.NormalizeIdentifier(field.Name)
		if relations[name] == nil {
			relations[name] = make(map[string]bool, 2)
		}
		relations[name][field.Schema+"."+field.Table] = true
	}
	var fields []InspectField
	for _, field := range all {
		if len(relations[r.Dialect.NormalizeIdentifier(field.Name)]) > 1 {
			fields = append(fields, field)
		}
	}
	return fields
}

// RelationRefsOf reads back the relations a statement resolved to, for a clause
// inspected after the statement was built rather than alongside it.
func RelationRefsOf(stmt *InspectStatement) []RelationRef {
	refs := make([]RelationRef, 0, len(stmt.Tables))
	for _, table := range stmt.Tables {
		ref := RelationRef{Table: table.Name, Schema: table.Schema}
		if table.Alias != nil {
			ref.Alias = *table.Alias
		}
		refs = append(refs, ref)
	}
	return refs
}

// ThroughVirtual rewrites a field read from a name the statement declared
// itself, a CTE or a derived table, into the column of the table that name
// returns: "SELECT s.c FROM (SELECT c FROM t) s" reads t.c. A field naming
// something else is returned as it stands, and one naming a virtual relation
// that returns no such column is dropped, since it resolves to nothing.
//
// subqueries are the statements behind those names in the order the statement
// declared them: the CTEs first, then the derived tables by alias.
func (r Resolver) ThroughVirtual(fields []InspectField, refs []RelationRef, s Scope, subqueries []InspectStatement) []InspectField {
	virtual := r.virtualFields(refs, s, subqueries)
	if len(virtual) == 0 {
		return fields
	}

	resolved := make([]InspectField, 0, len(fields))
	for _, field := range fields {
		underlying, ok := virtual[r.Dialect.NormalizeIdentifier(field.Table)]
		if !ok {
			resolved = append(resolved, field)
			continue
		}
		for _, source := range underlying {
			if r.Dialect.NormalizeIdentifier(source.Name) == r.Dialect.NormalizeIdentifier(field.Name) {
				resolved = append(resolved, InspectField{
					Name:   field.Name,
					Alias:  field.Alias,
					Table:  source.Table,
					Schema: source.Schema,
				})
				break
			}
		}
	}
	return resolved
}

// virtualFields maps each name the statement declared to the fields the
// statement behind it returns.
func (r Resolver) virtualFields(refs []RelationRef, s Scope, subqueries []InspectStatement) map[string][]InspectField {
	virtual := make(map[string][]InspectField, len(s.CTEs)+len(s.Subqueries))

	// A name the statement qualified is the real table even where a CTE
	// shadows the bare one, so its columns are not the CTE's.
	qualified := make(map[string]bool)
	for _, ref := range refs {
		if ref.Qualified {
			qualified[r.Dialect.NormalizeIdentifier(ref.Table)] = true
		}
	}

	for idx, cte := range s.CTEs {
		if idx >= len(subqueries) {
			break
		}
		name := r.Dialect.NormalizeIdentifier(cte.Table)
		if qualified[name] {
			continue
		}
		virtual[name] = subqueries[idx].Fields
	}

	// The derived tables were inspected in the order the FROM list names them,
	// which is the order refs is in. Pairing them by any other order, such as
	// the aliases sorted, maps an alias to another subquery's columns.
	next := len(s.CTEs)
	seen := make(map[string]bool, len(s.Subqueries))
	for _, ref := range refs {
		alias := ref.Alias
		if alias == "" {
			alias = ref.Table
		}
		if _, ok := s.Subqueries[alias]; !ok || seen[alias] {
			continue
		}
		seen[alias] = true
		if next >= len(subqueries) {
			break
		}
		virtual[r.Dialect.NormalizeIdentifier(alias)] = subqueries[next].Fields
		next++
	}

	return virtual
}

// Column resolves a column written with a relation prefix, "s.c", against the
// relations in scope. A prefix naming a relation the metadata holds no columns
// for is a CTE or a derived table: the field keeps that name, for
// ThroughVirtual to rewrite into whatever the name returns. A prefix matching
// no relation at all resolves to nothing.
func (r Resolver) Column(prefix, name string, refs []RelationRef) *InspectField {
	wanted := r.Dialect.NormalizeIdentifier(prefix)
	for _, ref := range refs {
		named := ref.Alias
		if named == "" {
			named = ref.Table
		}
		if r.Dialect.NormalizeIdentifier(named) != wanted {
			continue
		}
		for _, column := range TableFields(r.Meta, ref.Schema, ref.Table, r.Dialect) {
			if r.Dialect.NormalizeIdentifier(column.Name) == r.Dialect.NormalizeIdentifier(name) {
				return &InspectField{Name: column.Name, Table: ref.Table, Schema: ref.Schema}
			}
		}
		return &InspectField{Name: name, Table: ref.Table, Schema: ref.Schema}
	}
	return nil
}
