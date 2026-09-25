package core

// UnknownStatement is what an inspector returns for a statement it parsed but
// cannot classify. Permission checks read it as manage, so a statement nobody
// has taught us to read is refused rather than waved through as nothing.
func UnknownStatement() InspectStatement {
	return InspectStatement{Operation: InspectOpUnknown}
}

// TransactionStatement is what an inspector returns for a boundary of the
// transaction its own session runs in. It names no object, reads no row and
// writes none: what it decides is when the statements around it become
// visible, and each of those is inspected and priced on its own.
func TransactionStatement() InspectStatement {
	return InspectStatement{Operation: InspectOpTransaction}
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

// NestUnderUnknown reports reads as the nested statements of an unclassified
// one. A statement doing something the four row actions do not cover takes
// manage for that, and still whatever the rows themselves need.
//
// Passing no read is UnknownStatement, which is all a dispatcher that read
// nothing of the statement has to report.
func NestUnderUnknown(reads ...InspectStatement) InspectStatement {
	return InspectStatement{
		Operation:  InspectOpUnknown,
		Subqueries: reads,
	}
}

// AlsoPerforms records that a statement does something to its own tables that
// its operation does not name: an upsert rewrites the row it conflicts with,
// a REPLACE deletes it.
//
// It goes in Also rather than Subqueries: a subquery under a write is a value
// the write stored, which the see check reads as a column out of reach of
// masking, and these columns are written rather than read. Naming no column
// asks for the right on the whole table, which is what a row leaving it takes;
// an upsert names the columns it sets, so a role holding update on those
// columns still runs.
func AlsoPerforms(stmt *InspectStatement, op InspectOperation, fields []InspectField) {
	if stmt == nil || len(stmt.Tables) == 0 {
		return
	}
	tables := make([]InspectTable, len(stmt.Tables))
	copy(tables, stmt.Tables)
	stmt.Also = append(stmt.Also, InspectStatement{
		Operation: op,
		Tables:    tables,
		Fields:    fields,
	})
}

// AlsoReads records the relations a statement reads without writing them: the
// tables a multi-table UPDATE or DELETE joins against but does not target.
func AlsoReads(stmt *InspectStatement, tables []InspectTable) {
	if stmt == nil || len(tables) == 0 {
		return
	}
	stmt.Also = append(stmt.Also, InspectStatement{Operation: InspectOpSelect, Tables: tables})
}

// SplitWrite reports the tables a write changes and, on the statement, the
// ones it only reads. A statement whose fields name no table writes all of
// them, which is what an inspector that resolved no column must fall back to.
func SplitWrite(stmt *InspectStatement) {
	if stmt == nil {
		return
	}
	written := tablesNamedBy(stmt.Tables, stmt.Fields)
	if len(written) == 0 {
		return
	}
	AlsoReads(stmt, TablesExcept(stmt.Tables, written))
	stmt.Tables = written
}

// tablesNamedBy are the tables of all that some field belongs to.
func tablesNamedBy(all []InspectTable, fields []InspectField) []InspectTable {
	named := make(map[[2]string]bool, len(fields))
	for _, field := range fields {
		named[[2]string{field.Schema, field.Table}] = true
	}
	var held []InspectTable
	for _, table := range all {
		if named[[2]string{table.Schema, table.Name}] {
			held = append(held, table)
		}
	}
	return held
}

// AsFilter marks stmts as filters and returns them. See InspectStatement.Filter.
func AsFilter(stmts []InspectStatement) []InspectStatement {
	for i := range stmts {
		stmts[i].Filter = true
	}
	return stmts
}

// SalvageOrUnknown decides how much of what error recovery salvaged to trust.
//
// An error on the span's first token means the statement never began, so the
// salvage belongs to some other fragment: SQLite has no REVOKE, and
// "REVOKE SELECT ON t1 FROM bob" leaves a select on a table named bob. A
// salvage naming no table leaves a per-table check nothing to ask about, so it
// takes manage instead. An error anywhere else is a clause the grammar does
// not carry, such as SQLite's standalone WINDOW, and what the statement named
// still stands.
func SalvageOrUnknown(read InspectStatement, syntax *SyntaxErrors, from, to int) InspectStatement {
	switch {
	case !syntax.In(from, to):
		return read
	case syntax.At(from):
		return UnknownStatement()
	case len(read.Tables) > 0:
		return read
	default:
		return NestUnderUnknown(read)
	}
}

// DropVirtualTables strips, in place and throughout the tree, the tables naming
// a CTE the enclosing query declared. A CTE is a relation at any depth, so a
// subquery inspected without that scope reports one as a table resolving to no
// schema, which is refused for every role.
//
// Only a name the SQL wrote without a schema is dropped: a qualified one is
// the real table even where a CTE shadows the bare name, and dropping it would
// be a read nobody checks. normalize applies to both sides, so virtual may
// hold any casing.
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
			if table.Bare && declared[normalize(table.Name)] {
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
