package core

import "strings"

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

// virtualNames returns the normalized names in scope that resolve to something
// the statement declared rather than to a table.
func (r Resolver) virtualNames(s Scope) map[string]bool {
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
// check reads. An unqualified name the metadata does not know resolves to no
// schema, which is refused for every role. Only such a name can be virtual:
// dropping a qualified one would be a read nobody checks, so a qualified name
// keeps the schema it was written with whether or not the metadata has it.
func (r Resolver) Tables(refs []RelationRef, s Scope) []InspectTable {
	virtual := r.virtualNames(s)
	var tables []InspectTable
	seen := make(map[string]bool)

	for _, ref := range refs {
		if ref.Schema == "" && ref.Table == "" {
			continue
		}
		if !ref.Qualified && virtual[r.Dialect.NormalizeIdentifier(ref.Table)] {
			continue
		}

		schema := r.TableSchema(ref.Schema, ref.Table, ref.Qualified)

		key := schema + "." + ref.Table
		if seen[key] {
			continue
		}
		seen[key] = true

		table := InspectTable{
			Name:      ref.Table,
			Schema:    schema,
			Bare:      !ref.Qualified,
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

// TableSchema is the schema a named relation resolves in. An unqualified name
// the metadata does not know resolves to no schema: it could be a table in any
// schema, so a row right on the default one would be a guess. A qualified name
// keeps the schema it was written with whether or not the metadata has it,
// since dropping it would be a read nobody checks.
//
// A write resolves its target through this too. Defaulting there and refusing
// here made the answer depend on the verb: the same unknown name ran under a
// row right on one path and was unrunnable on the other.
func (r Resolver) TableSchema(schema, table string, qualified bool) string {
	if qualified || TableExistsInMetadata(r.Meta, schema, table, r.Dialect) {
		return schema
	}
	return ""
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
// outside the statement that declared it. A name left standing asks for a
// right on a relation nobody holds one on, so every statement carrying a WITH
// calls this.
func (r Resolver) DropCTETables(stmts []InspectStatement, ctes []RelationRef) {
	r.DropVirtual(stmts, r.virtualNames(Scope{CTEs: ctes}))
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

// ResolveCorrelated resolves what a subquery reads from a relation of the
// statement enclosing it, which its own inspection could not: only the FROM of
// the subquery itself was in scope there. "WHERE EXISTS (SELECT 1 FROM logs l
// WHERE l.email = u.email)" tests u.email, and the row count answers for it.
//
// A name still resolving to no relation after this is dropped: it carries no
// table, so there is nothing to check it against.
func (r Resolver) ResolveCorrelated(stmts []InspectStatement, outer []RelationRef) {
	for idx := range stmts {
		stmt := &stmts[idx]
		inScope := append(append([]RelationRef{}, outer...), RelationRefsOf(stmt)...)
		stmt.Where = r.resolveEach(stmt.Where, outer)
		r.ResolveCorrelated(stmt.Subqueries, inScope)
	}
}

// resolveEach retries the fields that named a relation nothing in their own
// statement matched, and drops the ones still naming none.
func (r Resolver) resolveEach(fields []InspectField, refs []RelationRef) []InspectField {
	kept := fields[:0]
	for _, field := range fields {
		if field.Schema != "" {
			kept = append(kept, field)
			continue
		}
		if resolved := r.Column(field.Table, field.Name, refs); resolved != nil && resolved.Schema != "" {
			kept = append(kept, *resolved)
		}
	}
	return kept
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
			// The name to match is the one the subquery hands up, which an
			// alias replaces: "(SELECT email AS e FROM users) s" gives s.e.
			if r.Dialect.NormalizeIdentifier(outputName(source)) != r.Dialect.NormalizeIdentifier(field.Name) {
				continue
			}
			// The column carries its own name from here on, since that is the
			// name a rule hides. What the outer statement calls it stays as
			// the alias, which is the name its result column comes out under.
			rewritten := InspectField{Name: source.Name, Table: source.Table, Schema: source.Schema}
			if outer := outputName(field); r.Dialect.NormalizeIdentifier(outer) != r.Dialect.NormalizeIdentifier(source.Name) {
				rewritten.Alias = &outer
			}
			resolved = append(resolved, rewritten)
			break
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

// UnqualifiedColumn resolves a column written without a relation prefix
// against the relations in scope, taking the first that holds it.
//
// A name no relation holds under that spelling is tried again ignoring case,
// which SQLite and MySQL do whichever way the name is written. A name still
// unplaced is asked of each relation in turn, which is what reaches a CTE or
// a derived table: only the scope says what those return.
func (r Resolver) UnqualifiedColumn(name string, refs []RelationRef, s Scope) *InspectField {
	if field := r.firstHolding(name, refs, false); field != nil {
		return field
	}
	if field := r.firstHolding(name, refs, true); field != nil {
		return field
	}
	// A relation the statement declared holds it under a name of its own, and
	// only the scope says what a CTE or a derived table returns.
	for _, ref := range refs {
		if field := r.columnOf(ref, name, nil, s); field != nil {
			return field
		}
	}
	return nil
}

func (r Resolver) firstHolding(name string, refs []RelationRef, foldCase bool) *InspectField {
	wanted := r.Dialect.NormalizeIdentifier(name)
	for _, ref := range refs {
		for _, column := range TableFields(r.Meta, ref.Schema, ref.Table, r.Dialect) {
			held := r.Dialect.NormalizeIdentifier(column.Name)
			if held == wanted || (foldCase && strings.EqualFold(held, wanted)) {
				return &InspectField{Name: column.Name, Table: ref.Table, Schema: ref.Schema}
			}
		}
	}
	return nil
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
		wantedColumn := r.Dialect.NormalizeIdentifier(name)
		for _, column := range TableFields(r.Meta, ref.Schema, ref.Table, r.Dialect) {
			held := r.Dialect.NormalizeIdentifier(column.Name)
			if held == wantedColumn || strings.EqualFold(held, wantedColumn) {
				return &InspectField{Name: column.Name, Table: ref.Table, Schema: ref.Schema}
			}
		}
		return &InspectField{Name: name, Table: ref.Table, Schema: ref.Schema}
	}
	// The prefix may name a relation of an enclosing statement, which is not in
	// scope here. The field is kept under that name for ResolveCorrelated.
	return &InspectField{Name: name, Table: prefix}
}

// SelectColumn resolves a column an expression names, "c" or "t.c", to the
// table it is read from.
//
// A prefix decides on its own: a name in scope is that relation's column
// whether or not the metadata lists it. Without one the relations are tried in
// the order the statement named them, and a name none of them holds is
// attributed to the one real table in scope, where there is exactly one. It
// resolves to nothing otherwise, which refuses rather than guesses.
//
// alias is the name the statement gives the result column, and belongs to the
// field only where the expression resolves to this column alone.
func (r Resolver) SelectColumn(name, prefix string, alias *string, refs []RelationRef, s Scope) *InspectField {
	if prefix != "" {
		return r.prefixedColumn(name, prefix, alias, refs, s)
	}
	for _, ref := range refs {
		if field := r.columnOf(ref, name, alias, s); field != nil {
			return field
		}
	}
	return r.soleRelationColumn(name, alias, refs)
}

// prefixedColumn resolves "t.c", where t names a relation the statement has in
// scope. The relation answers even where the metadata has no such column: the
// name says which table to ask about, and a column the metadata is missing is
// what a check has to refuse rather than skip.
func (r Resolver) prefixedColumn(name, prefix string, alias *string, refs []RelationRef, s Scope) *InspectField {
	for _, ref := range refs {
		if !r.named(ref, prefix) {
			continue
		}
		if cte, ok := r.matchCTE(s.CTEs, ref); ok {
			resolved := r.CTEColumn(r.Dialect.NormalizeIdentifier(name), s.CTEResults[r.Dialect.NormalizeIdentifier(cte.Table)])
			if resolved != nil {
				resolved.Alias = alias
			}
			return resolved
		}
		return &InspectField{
			Name:   r.canonicalName(name, ref),
			Alias:  alias,
			Table:  ref.Table,
			Schema: ref.Schema,
		}
	}
	return nil
}

// columnOf reports the field where ref holds a column of that name, whether
// ref is a CTE, a derived table or a table of the metadata.
func (r Resolver) columnOf(ref RelationRef, name string, alias *string, s Scope) *InspectField {
	// A name can be a CTE and a derived table at once, and the FROM decides
	// which one the statement reads. Where the CTE does not hold the column,
	// the relation is tried as what else it could be rather than given up on.
	if cte, ok := r.matchCTE(s.CTEs, ref); ok && holds(cte.Columns, name, r.Dialect) {
		if resolved := r.CTEColumn(r.Dialect.NormalizeIdentifier(name), s.CTEResults[r.Dialect.NormalizeIdentifier(cte.Table)]); resolved != nil {
			resolved.Alias = alias
			return resolved
		}
	}

	if ref.Schema == "" {
		if columns, ok := s.Subqueries[ref.Table]; ok {
			if column := hold(columns, name, r.Dialect); column != nil {
				return &InspectField{Name: column.Name, Alias: alias, Table: ref.Table, Schema: r.Meta.DefaultSchema}
			}
		}
		return nil
	}

	for _, column := range TableFields(r.Meta, ref.Schema, ref.Table, r.Dialect) {
		if sameIdentifier(column.Name, name, r.Dialect) {
			return &InspectField{Name: column.Name, Alias: alias, Table: ref.Table, Schema: ref.Schema}
		}
	}
	return nil
}

// soleRelationColumn attributes a name no relation holds to the only real
// table in scope. A column the metadata has not caught up with is that table's,
// and there is nowhere else it could be from. Where more than one table could
// hold it, it resolves to nothing, which refuses rather than picks one.
func (r Resolver) soleRelationColumn(name string, alias *string, refs []RelationRef) *InspectField {
	var real []RelationRef
	for _, ref := range refs {
		if ref.Schema != "" {
			real = append(real, ref)
		}
	}
	if len(real) != 1 {
		return nil
	}
	field := InspectField{Name: name, Alias: alias, Table: real[0].Table, Schema: real[0].Schema}
	// A table the metadata does not describe keeps an empty schema, which is
	// what refuses the statement rather than checking it against nothing.
	if !TableExistsInMetadata(r.Meta, real[0].Schema, real[0].Table, r.Dialect) {
		field.Schema = ""
	}
	return &field
}

// canonicalName is the column's name as the metadata spells it, which keeps
// the case a quoted name in the statement may not have.
func (r Resolver) canonicalName(name string, ref RelationRef) string {
	for _, column := range TableFields(r.Meta, ref.Schema, ref.Table, r.Dialect) {
		if sameIdentifier(column.Name, name, r.Dialect) {
			return column.Name
		}
	}
	return r.Dialect.NormalizeIdentifier(name)
}

func (r Resolver) named(ref RelationRef, name string) bool {
	called := ref.Alias
	if called == "" {
		called = ref.Table
	}
	return r.Dialect.NormalizeIdentifier(called) == r.Dialect.NormalizeIdentifier(name)
}

func holds(columns []Column, name string, dialect SQLDialect) bool {
	return hold(columns, name, dialect) != nil
}

func hold(columns []Column, name string, dialect SQLDialect) *Column {
	for idx, column := range columns {
		if sameIdentifier(column.Name, name, dialect) {
			return &columns[idx]
		}
	}
	return nil
}

// sameIdentifier compares two names as the dialect spells them, then ignoring
// case: SQLite and MySQL match a column name case-insensitively whichever way
// it is written, so a quoted "Email" is the email column.
func sameIdentifier(a, b string, dialect SQLDialect) bool {
	left, right := dialect.NormalizeIdentifier(a), dialect.NormalizeIdentifier(b)
	return left == right || strings.EqualFold(left, right)
}
