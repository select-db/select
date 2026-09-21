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

// AlsoPerforms records that a statement does something to its own tables that
// its operation does not name: an upsert rewrites the row it conflicts with,
// a REPLACE deletes it. The permission check already walks nested statements,
// so the second right is asked for by reporting one that needs it rather than
// by teaching the check that an insert is sometimes two things.
func AlsoPerforms(stmt *InspectStatement, op InspectOperation) {
	if stmt == nil || len(stmt.Tables) == 0 {
		return
	}
	tables := make([]InspectTable, len(stmt.Tables))
	copy(tables, stmt.Tables)
	stmt.Subqueries = append(stmt.Subqueries, InspectStatement{Operation: op, Tables: tables})
}

// AsFilter marks stmts as filters and returns them. See InspectStatement.Filter.
func AsFilter(stmts []InspectStatement) []InspectStatement {
	for i := range stmts {
		stmts[i].Filter = true
	}
	return stmts
}

// NestUnderUnknownIfUnreadable reports read under an unclassified statement
// when what error recovery salvaged is not the statement the caller wrote.
//
// Two shapes qualify. A salvage that named no table leaves a per-table check
// nothing to ask about, so it would run on a policy granting nothing. A
// salvage from a statement that never began, which the parser stumbling over
// its first token is what says, names tables belonging to whatever fragment
// recovery found: SQLite has no REVOKE, so "REVOKE SELECT ON t1 FROM bob"
// leaves a select on a table named bob.
//
// An error later in the span is a clause the grammar does not carry, such as
// SQLite's standalone WINDOW. The statement's own head parsed, what it named
// still stands, and nesting it would refuse ordinary work.
func NestUnderUnknownIfUnreadable(read InspectStatement, syntax *SyntaxErrors, from, to int) InspectStatement {
	if !syntax.In(from, to) {
		return read
	}
	if syntax.AtStart(from) {
		// The statement never began, so what follows is a fragment of
		// something else. Keeping its read would check a table the caller
		// never named, and report that name back as the reason.
		return UnknownStatement()
	}
	if len(read.Tables) > 0 {
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
