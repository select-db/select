package testutil

import core "github.com/selectDb/dialect/core"

// PermCase is one statement and the rights it requires. The expectation is
// written before anything is measured: a case states what SQL means, not what
// the inspectors currently report.
//
// Needs is the exact set. Every right in it is necessary, which the runner
// checks by withholding each one in turn, and together they are sufficient.
// An empty Needs means the statement requires nothing, which only a statement
// naming no object can.
//
// Op is the operation the case expects the inspector to read. A case needing
// manage alone is met by a statement floored to unknown, so naming the
// operation is what tells a statement the inspector understood from one it
// gave up on. Leave it out where the floor is the right answer, as it is for
// SQL no dialect here parses.
//
// Cases here MUST hold for every dialect that accepts the SQL. A statement
// only some dialects parse goes in the table for those dialects.
type PermCase struct {
	Name  string
	SQL   string
	Needs []Right
	Op    core.InspectOperation
	Why   string
}

// t1, t2 and t3 are the tables of GetInspectTestMetadata, named here so a case
// reads as a sentence.
var (
	t1 = func(action string) Right { return Right{Action: action, Schema: "main", Table: "t1"} }
	t2 = func(action string) Right { return Right{Action: action, Schema: "main", Table: "t2"} }
	t3 = func(action string) Right { return Right{Action: action, Schema: "other", Table: "t3"} }
)

// PermCasesFor are the cases a dialect runs: what every dialect parses, plus
// each table for SQL this one accepts. A dialect added later names itself here
// once rather than in a condition beside every table.
func PermCasesFor(dialect string) []PermCase {
	cases := GetPermCasesEveryDialect()
	if dialect != "mysql" {
		cases = append(cases, GetPermCasesPostgreSQLAndSQLite()...)
	}
	if dialect != "postgresql" {
		cases = append(cases, GetPermCasesMySQLAndSQLite()...)
	}
	if dialect == "mysql" {
		cases = append(cases, GetPermCasesMySQL()...)
	}
	return cases
}

// GetPermCasesEveryDialect are statements all three dialects parse, with the
// rights each requires.
func GetPermCasesEveryDialect() []PermCase {
	return []PermCase{
		// --- the four data statements, which are the floor every other case
		// is measured against
		{
			Name:  "a select reads",
			SQL:   "SELECT c1 FROM t1",
			Needs: []Right{t1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "it returns rows of t1",
		},
		{
			Name:  "an insert writes",
			SQL:   "INSERT INTO t1 (c1) VALUES (1)",
			Needs: []Right{t1(core.ActionInsert)},
			Op:    core.InspectOpInsert,
			Why:   "it adds a row and reads nothing",
		},
		{
			Name:  "an update writes",
			SQL:   "UPDATE t1 SET c1 = 1",
			Needs: []Right{t1(core.ActionUpdate)},
			Op:    core.InspectOpUpdate,
			Why:   "it changes rows and returns none",
		},
		{
			Name:  "a delete writes",
			SQL:   "DELETE FROM t1",
			Needs: []Right{t1(core.ActionDelete)},
			Op:    core.InspectOpDelete,
			Why:   "it removes rows and returns none",
		},

		// --- a write carrying a read
		{
			Name:  "an insert from a select reads what it copies",
			SQL:   "INSERT INTO t1 (c1) SELECT c1 FROM t2",
			Needs: []Right{t1(core.ActionInsert), t2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "holding insert alone would copy t2 into a table the role can read",
		},
		{
			Name:  "an update from a subquery reads it",
			SQL:   "UPDATE t1 SET c1 = (SELECT c1 FROM t2)",
			Needs: []Right{t1(core.ActionUpdate), t2(core.ActionSelect)},
			Op:    core.InspectOpUpdate,
			Why:   "the value written is read out of t2",
		},
		{
			Name:  "a delete filtered by a subquery reads it",
			SQL:   "DELETE FROM t1 WHERE c1 IN (SELECT c1 FROM t2)",
			Needs: []Right{t1(core.ActionDelete), t2(core.ActionSelect)},
			Op:    core.InspectOpDelete,
			Why:   "which rows go depends on what t2 holds",
		},

		// --- where a read hides. Each names a second relation somewhere the
		// FROM list does not, and a right on it is what says the inspector
		// found it.
		{
			Name:  "two tables in the FROM list",
			SQL:   "SELECT t1.c1, t2.c3 FROM t1, t2",
			Needs: []Right{t1(core.ActionSelect), t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "both are read, and a check that stops at the first reads t2 for free",
		},
		{
			Name:  "a join",
			SQL:   "SELECT t1.c1 FROM t1 JOIN t2 ON t1.c1 = t2.c1",
			Needs: []Right{t1(core.ActionSelect), t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "t2 decides which rows come back even where no column of it is returned",
		},
		{
			Name:  "a join across schemas",
			SQL:   "SELECT t1.c1 FROM t1 JOIN other.t3 ON t1.c1 = other.t3.c1",
			Needs: []Right{t1(core.ActionSelect), t3(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "a right on main.t3 is not a right on other.t3",
		},
		{
			Name:  "a CTE body",
			SQL:   "WITH x AS (SELECT c1 FROM t2) SELECT c1 FROM x",
			Needs: []Right{t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "x is not a table, and what it reads is t2",
		},
		{
			Name:  "a derived table",
			SQL:   "SELECT q.c1 FROM (SELECT c1 FROM t2) q",
			Needs: []Right{t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "the alias is not a table, and what it reads is t2",
		},
		{
			Name:  "a scalar subquery in the select list",
			SQL:   "SELECT c1, (SELECT c1 FROM t2 LIMIT 1) AS s FROM t1",
			Needs: []Right{t1(core.ActionSelect), t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "its value reaches the row whether or not the FROM names t2",
		},
		{
			Name:  "a subquery in WHERE",
			SQL:   "SELECT c1 FROM t1 WHERE c1 IN (SELECT c1 FROM t2)",
			Needs: []Right{t1(core.ActionSelect), t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "which rows come back is an answer about t2",
		},
		{
			Name:  "a subquery in HAVING",
			SQL:   "SELECT c1 FROM t1 GROUP BY c1 HAVING count(*) > (SELECT count(*) FROM t2)",
			Needs: []Right{t1(core.ActionSelect), t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "which groups survive is an answer about t2",
		},
		{
			Name:  "a subquery in ORDER BY",
			SQL:   "SELECT c1 FROM t1 ORDER BY (SELECT count(*) FROM t2)",
			Needs: []Right{t1(core.ActionSelect), t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "the order the rows come back in is an answer about t2",
		},
		{
			Name:  "each branch of a union",
			SQL:   "SELECT c1 FROM t1 UNION SELECT c1 FROM t2",
			Needs: []Right{t1(core.ActionSelect), t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "a branch nobody checked is a table read without a right",
		},
		{
			Name:  "a correlated EXISTS",
			SQL:   "SELECT c1 FROM t1 WHERE EXISTS (SELECT 1 FROM t2 WHERE t2.c1 = t1.c1)",
			Needs: []Right{t1(core.ActionSelect), t2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "whether a row survives is an answer about t2",
		},

		// --- statements that name no table and change nothing
		{
			// A connection check is the commonest statement there is, and it
			// reports nothing about any table, so it asks for nothing. That
			// also makes it the one case that must run under a policy
			// granting nothing at all.
			Name:  "a select of a constant names no table",
			SQL:   "SELECT 1",
			Needs: nil,
			Why:   "it reads no table, so there is nothing to hold a right on",
		},

		// --- schema changes, which are the administration right
		{
			Name:  "creating a table is administration",
			SQL:   "CREATE TABLE t9 (c1 INTEGER)",
			Needs: []Right{Manage},
			Op:    core.InspectOpCreate,
			Why:   "the schema is not data",
		},
		{
			Name:  "creating a table from a query reads the query",
			SQL:   "CREATE TABLE t9 AS SELECT c1 FROM t1",
			Needs: []Right{Manage, t1(core.ActionSelect)},
			Op:    core.InspectOpCreate,
			Why:   "manage covers making the table, not reading t1 into it",
		},
		{
			Name:  "creating a view reads what it selects",
			SQL:   "CREATE VIEW v1 AS SELECT c1 FROM t1",
			Needs: []Right{Manage, t1(core.ActionSelect)},
			Op:    core.InspectOpCreate,
			Why:   "the view body is a read that anyone selecting the view inherits",
		},
		{
			Name:  "altering a table is administration",
			SQL:   "ALTER TABLE t1 ADD COLUMN c9 INTEGER",
			Needs: []Right{Manage},
			Op:    core.InspectOpAlter,
			Why:   "it changes the schema",
		},
		{
			Name:  "dropping a table is administration",
			SQL:   "DROP TABLE t1",
			Needs: []Right{Manage},
			Op:    core.InspectOpDrop,
			Why:   "delete removes rows, drop removes the table",
		},

		// --- rights and session statements
		{
			Name:  "granting a right is administration",
			SQL:   "GRANT SELECT ON t1 TO bob",
			Needs: []Right{Manage},
			Why:   "it hands a right to somebody else",
		},
		{
			Name:  "revoking a right is administration",
			SQL:   "REVOKE SELECT ON t1 FROM bob",
			Needs: []Right{Manage},
			Why:   "it takes a right away from somebody else",
		},
	}
}

// GetPermCasesPostgreSQLAndSQLite are statements those two parse and MySQL
// does not: the standard upsert, written ON CONFLICT.
func GetPermCasesPostgreSQLAndSQLite() []PermCase {
	return []PermCase{
		{
			Name:  "an upsert that updates on conflict also updates",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) DO UPDATE SET c2 = 'x'",
			Needs: []Right{t1(core.ActionInsert), t1(core.ActionUpdate)},
			Op:    core.InspectOpInsert,
			Why:   "a role holding insert alone would rewrite a row it may not update",
		},
		{
			Name:  "an upsert that does nothing on conflict only inserts",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) DO NOTHING",
			Needs: []Right{t1(core.ActionInsert)},
			Op:    core.InspectOpInsert,
			Why:   "the conflicting row is left exactly as it was",
		},
		{
			Name:  "RETURNING hands rows back, which is a read",
			SQL:   "UPDATE t1 SET c1 = 1 RETURNING c2",
			Needs: []Right{t1(core.ActionUpdate), t1(core.ActionSelect)},
			Op:    core.InspectOpUpdate,
			Why:   "a role holding update alone would read t1 by writing to it",
		},
		{
			Name:  "a delete returning what it removed",
			SQL:   "DELETE FROM t1 RETURNING c2",
			Needs: []Right{t1(core.ActionDelete), t1(core.ActionSelect)},
			Op:    core.InspectOpDelete,
			Why:   "the rows come back to the caller on their way out",
		},
		{
			Name:  "an insert returning what it wrote",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) RETURNING c2",
			Needs: []Right{t1(core.ActionInsert), t1(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "c2 is whatever the table put there, which the caller did not supply",
		},
		{
			Name:  "UPDATE ... FROM reads the table it joins against",
			SQL:   "UPDATE t1 SET c1 = 2 FROM t2 WHERE t1.c1 = t2.c1",
			Needs: []Right{t1(core.ActionUpdate), t2(core.ActionSelect)},
			Op:    core.InspectOpUpdate,
			Why:   "t2 is read to decide which rows of t1 change, and it is not written",
		},
		{
			Name:  "an upsert whose conflict predicate reads another table",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) WHERE c1 IN (SELECT c1 FROM t2) DO NOTHING",
			Needs: []Right{t1(core.ActionInsert), t2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "whether the row is inserted is an answer about t2",
		},
		{
			Name:  "an upsert reading another table to update from it",
			SQL:   "INSERT INTO t1 (c1) SELECT c1 FROM t2 ON CONFLICT (c1) DO UPDATE SET c2 = 'x'",
			Needs: []Right{t1(core.ActionInsert), t1(core.ActionUpdate), t2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "it inserts, it rewrites what was there, and it reads t2 to do it",
		},
	}
}

// GetPermCasesMySQLAndSQLite are statements those two parse and PostgreSQL
// does not: REPLACE, which is an insert that first deletes whatever conflicts.
func GetPermCasesMySQLAndSQLite() []PermCase {
	return []PermCase{
		{
			Name:  "REPLACE deletes the row it conflicts with",
			SQL:   "REPLACE INTO t1 (c1) VALUES (1)",
			Needs: []Right{t1(core.ActionInsert), t1(core.ActionDelete)},
			Op:    core.InspectOpInsert,
			Why:   "the row that was there is gone, so insert alone destroys data",
		},
		{
			Name:  "REPLACE from a select reads what it copies",
			SQL:   "REPLACE INTO t1 (c1) SELECT c1 FROM t2",
			Needs: []Right{t1(core.ActionInsert), t1(core.ActionDelete), t2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "it destroys rows of t1 and reads t2 to do it",
		},
	}
}

// GetPermCasesMySQL are statements MySQL alone parses: its own spelling of the
// upsert.
func GetPermCasesMySQL() []PermCase {
	return []PermCase{
		{
			Name:  "ON DUPLICATE KEY UPDATE also updates",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON DUPLICATE KEY UPDATE c2 = 'x'",
			Needs: []Right{t1(core.ActionInsert), t1(core.ActionUpdate)},
			Op:    core.InspectOpInsert,
			Why:   "a role holding insert alone would rewrite a row it may not update",
		},
		{
			Name:  "a multi-table UPDATE writes what it sets and reads the rest",
			SQL:   "UPDATE t1 JOIN t2 ON t1.c1 = t2.c1 SET t1.c1 = 2",
			Needs: []Right{t1(core.ActionUpdate), t2(core.ActionSelect)},
			Op:    core.InspectOpUpdate,
			Why:   "t2 is joined against, so update on it is a right the statement does not need",
		},
		{
			Name:  "a multi-table UPDATE writing both tables",
			SQL:   "UPDATE t1 JOIN t2 ON t1.c1 = t2.c1 SET t1.c1 = 2, t2.c3 = 'x'",
			Needs: []Right{t1(core.ActionUpdate), t2(core.ActionUpdate)},
			Op:    core.InspectOpUpdate,
			Why:   "both are named by the SET list, so both are written",
		},
		{
			Name:  "a multi-table DELETE reads what it joins against",
			SQL:   "DELETE t1 FROM t1 JOIN t2 ON t1.c1 = t2.c1",
			Needs: []Right{t1(core.ActionDelete), t2(core.ActionSelect)},
			Op:    core.InspectOpDelete,
			Why:   "which rows of t1 go is an answer about t2, and only t1 is deleted from",
		},
		{
			Name:  "ON DUPLICATE KEY UPDATE reading another table",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON DUPLICATE KEY UPDATE c2 = (SELECT c3 FROM t2)",
			Needs: []Right{t1(core.ActionInsert), t1(core.ActionUpdate), t2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "the value it writes over the old one is read out of t2",
		},
	}
}
