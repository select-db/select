package core

// UnknownStatement is what an inspector returns for a statement it parsed but
// cannot classify. Permission checks read it as manage, so a statement nobody
// has taught us to read is refused rather than waved through as nothing.
func UnknownStatement() InspectStatement {
	return InspectStatement{Operation: InspectOpUnknown}
}

// OrUnknown returns *stmt, or an unknown statement when stmt is nil. Inspectors
// call it at the one point where a parsed statement becomes a result, so a
// dispatcher that falls through cannot drop the statement on the floor.
func OrUnknown(stmt *InspectStatement) InspectStatement {
	if stmt == nil {
		return UnknownStatement()
	}
	return *stmt
}

// NestUnderUnknown reports read as the nested statement of an unclassified
// one. A statement doing something the four row actions do not cover takes
// manage for that, and still whatever the rows themselves need.
func NestUnderUnknown(read InspectStatement) InspectStatement {
	return InspectStatement{
		Operation:  InspectOpUnknown,
		Subqueries: []InspectStatement{read},
	}
}

// AsFilter marks stmts as filters and returns them. See InspectStatement.Filter.
func AsFilter(stmts []InspectStatement) []InspectStatement {
	for i := range stmts {
		stmts[i].Filter = true
	}
	return stmts
}

// NestUnderUnknownIfUnreadable reports read under an unclassified statement
// when the parser stumbled over its span and it named no table: a per-table
// check has nothing to ask about there, so what error recovery salvaged would
// run on a policy granting nothing.
func NestUnderUnknownIfUnreadable(read InspectStatement, syntax *SyntaxErrors, from, to int) InspectStatement {
	if len(read.Tables) > 0 || !syntax.In(from, to) {
		return read
	}
	return NestUnderUnknown(read)
}

// DropVirtualTables strips, in place and throughout the tree, the tables naming
// a CTE the enclosing query declared. A CTE is a relation at any depth, so a
// subquery inspected without that scope reports one as a table resolving to no
// schema, which is refused for every role.
//
// Only a name that resolved to no schema is dropped: a qualified one is the
// real table even where a CTE shadows the bare name, and dropping it would be
// a read nobody checks. normalize applies to both sides, so virtual may hold
// any casing.
func DropVirtualTables(stmts []InspectStatement, virtual map[string]bool, normalize func(string) string) {
	if len(virtual) == 0 {
		return
	}
	declared := make(map[string]bool, len(virtual))
	for name := range virtual {
		declared[normalize(name)] = true
	}
	dropDeclaredTables(stmts, declared, normalize)
}

func dropDeclaredTables(stmts []InspectStatement, declared map[string]bool, normalize func(string) string) {
	for idx := range stmts {
		stmt := &stmts[idx]
		kept := stmt.Tables[:0]
		for _, table := range stmt.Tables {
			if table.Schema == "" && declared[normalize(table.Name)] {
				continue
			}
			kept = append(kept, table)
		}
		stmt.Tables = kept
		dropDeclaredTables(stmt.Subqueries, declared, normalize)
	}
}

// CTEScope returns the CTE names the body at idx can refer to. A plain WITH
// exposes only the CTEs declared before this one, so a name declared later is
// still the real table: PostgreSQL runs "WITH a AS (SELECT c1 FROM b), b AS
// (...)" against the table b. WITH RECURSIVE exposes every name in the clause,
// which is what lets a CTE refer to itself. idx is clamped rather than allowed
// to panic, since the caller is a permission check.
func CTEScope(names []string, idx int, recursive bool) map[string]bool {
	visible := names
	if !recursive {
		visible = names[:min(idx, len(names))]
	}
	scope := make(map[string]bool, len(visible))
	for _, name := range visible {
		scope[name] = true
	}
	return scope
}
