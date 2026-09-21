package testutil

import (
	"slices"

	core "github.com/selectDb/dialect/core"
)

// PermCase is one statement and the rights it requires. The expectation is
// written before anything is measured: a case states what SQL means, not what
// the inspectors currently report.
//
// Needs is the exact set: every right in it is necessary, which the runner
// checks by withholding each one in turn, and together they are sufficient.
//
// Op is the operation the case expects the inspector to read, which is what
// separates a statement it understood from one it gave up on. Leave it out
// where the floor is the right answer.
//
// On names the dialects that parse the SQL. None means all of them.
type PermCase struct {
	Name  string
	SQL   string
	Needs []Right
	Op    core.InspectOperation
	On    []string
	Why   string
}

// The tables of GetInspectTestMetadata. mainT1 and mainT2 share a schema and
// otherT3 does not, which is what the cross-schema cases turn on.
var (
	mainT1  = func(action string) Right { return Right{Action: action, Schema: "main", Table: "t1"} }
	mainT2  = func(action string) Right { return Right{Action: action, Schema: "main", Table: "t2"} }
	otherT3 = func(action string) Right { return Right{Action: action, Schema: "other", Table: "t3"} }
)

// PermCasesFor are the cases a dialect parses.
func PermCasesFor(dialect string) []PermCase {
	var cases []PermCase
	for _, testCase := range permCases() {
		if len(testCase.On) == 0 || slices.Contains(testCase.On, dialect) {
			cases = append(cases, testCase)
		}
	}
	return cases
}

// permCases is every case, each naming the dialects that parse it. A case
// naming none holds for all of them, so a dialect added later opts into the
// rest one case at a time rather than inheriting whatever a condition did not
// exclude.
func permCases() []PermCase {
	return []PermCase{
		// --- the four data statements, which are the floor every other case
		// is measured against
		{
			Name:  "a select reads",
			SQL:   "SELECT c1 FROM t1",
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "it returns rows of t1",
		},
		{
			Name:  "an insert writes",
			SQL:   "INSERT INTO t1 (c1) VALUES (1)",
			Needs: []Right{mainT1(core.ActionInsert)},
			Op:    core.InspectOpInsert,
			Why:   "it adds a row and reads nothing",
		},
		{
			Name:  "an update writes",
			SQL:   "UPDATE t1 SET c1 = 1",
			Needs: []Right{mainT1(core.ActionUpdate)},
			Op:    core.InspectOpUpdate,
			Why:   "it changes rows and returns none",
		},
		{
			Name:  "a delete writes",
			SQL:   "DELETE FROM t1",
			Needs: []Right{mainT1(core.ActionDelete)},
			Op:    core.InspectOpDelete,
			Why:   "it removes rows and returns none",
		},

		// --- a write carrying a read
		{
			Name:  "an insert from a select reads what it copies",
			SQL:   "INSERT INTO t1 (c1) SELECT c1 FROM t2",
			Needs: []Right{mainT1(core.ActionInsert), mainT2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "holding insert alone would copy t2 into a table the role can read",
		},
		{
			Name:  "an update from a subquery reads it",
			SQL:   "UPDATE t1 SET c1 = (SELECT c1 FROM t2)",
			Needs: []Right{mainT1(core.ActionUpdate), mainT2(core.ActionSelect)},
			Op:    core.InspectOpUpdate,
			Why:   "the value written is read out of t2",
		},
		{
			Name:  "a delete filtered by a subquery reads it",
			SQL:   "DELETE FROM t1 WHERE c1 IN (SELECT c1 FROM t2)",
			Needs: []Right{mainT1(core.ActionDelete), mainT2(core.ActionSelect)},
			Op:    core.InspectOpDelete,
			Why:   "which rows go depends on what t2 holds",
		},

		// --- where a read hides. Each names a second relation somewhere the
		// FROM list does not, and a right on it is what says the inspector
		// found it.
		{
			Name:  "two tables in the FROM list",
			SQL:   "SELECT t1.c1, t2.c3 FROM t1, t2",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "both are read, and a check that stops at the first reads t2 for free",
		},
		{
			Name:  "a join",
			SQL:   "SELECT t1.c1 FROM t1 JOIN t2 ON t1.c1 = t2.c1",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "t2 decides which rows come back even where no column of it is returned",
		},
		{
			Name:  "a join across schemas",
			SQL:   "SELECT t1.c1 FROM t1 JOIN other.t3 ON t1.c1 = other.t3.c1",
			Needs: []Right{mainT1(core.ActionSelect), otherT3(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "a right on main.t3 is not a right on other.t3",
		},
		{
			Name:  "a CTE body",
			SQL:   "WITH x AS (SELECT c1 FROM t2) SELECT c1 FROM x",
			Needs: []Right{mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "x is not a table, and what it reads is t2",
		},
		{
			Name:  "a derived table",
			SQL:   "SELECT q.c1 FROM (SELECT c1 FROM t2) q",
			Needs: []Right{mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "the alias is not a table, and what it reads is t2",
		},
		{
			Name:  "a scalar subquery in the select list",
			SQL:   "SELECT c1, (SELECT c1 FROM t2 LIMIT 1) AS s FROM t1",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "its value reaches the row whether or not the FROM names t2",
		},
		{
			Name:  "a subquery in WHERE",
			SQL:   "SELECT c1 FROM t1 WHERE c1 IN (SELECT c1 FROM t2)",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "which rows come back is an answer about t2",
		},
		{
			Name:  "a subquery in HAVING",
			SQL:   "SELECT c1 FROM t1 GROUP BY c1 HAVING count(*) > (SELECT count(*) FROM t2)",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "which groups survive is an answer about t2",
		},
		{
			Name:  "a subquery in ORDER BY",
			SQL:   "SELECT c1 FROM t1 ORDER BY (SELECT count(*) FROM t2)",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "the order the rows come back in is an answer about t2",
		},
		{
			Name:  "each branch of a union",
			SQL:   "SELECT c1 FROM t1 UNION SELECT c1 FROM t2",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "a branch nobody checked is a table read without a right",
		},
		{
			Name:  "a correlated EXISTS",
			SQL:   "SELECT c1 FROM t1 WHERE EXISTS (SELECT 1 FROM t2 WHERE t2.c1 = t1.c1)",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "whether a row survives is an answer about t2",
		},

		// --- how a relation is named. The rule is one relation, however it is
		// spelled, and a name that resolves to nothing is refused rather than
		// guessed at.
		{
			Name:  "a schema-qualified name",
			SQL:   "SELECT c1 FROM main.t1",
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "main.t1 and t1 are the same table",
		},
		{
			Name:  "a name in another schema",
			SQL:   "SELECT c1 FROM other.t3",
			Needs: []Right{otherT3(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "a right on main is not a right on other",
		},
		{
			Name:  "an alias",
			SQL:   "SELECT x.c1 FROM t1 AS x",
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "the alias is a name for t1, not a relation of its own",
		},
		{
			Name:  "an alias named after another table",
			SQL:   "SELECT x.c1 FROM t1 AS x, t2 AS t1",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "calling t2 by the name t1 must not hide either of them",
		},
		{
			Name:  "a CTE named after a table",
			SQL:   "WITH t1 AS (SELECT c1 FROM t2) SELECT c1 FROM t1",
			Needs: []Right{mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "the statement reads the CTE, so the table it shadows is never touched",
		},
		{
			Name:  "a qualified name a CTE shadows",
			SQL:   "WITH t1 AS (SELECT c1 FROM t2) SELECT c1 FROM main.t1",
			Needs: []Right{mainT1(core.ActionSelect), mainT2(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "qualifying reaches past the CTE to the real table, and the body still reads t2",
		},
		{
			Name:  "a name in another case",
			SQL:   "SELECT C1 FROM T1",
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "every dialect here folds an unquoted name, so T1 is t1",
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
			Needs: []Right{Manage, mainT1(core.ActionSelect)},
			Op:    core.InspectOpCreate,
			Why:   "manage covers making the table, not reading t1 into it",
		},
		{
			Name:  "creating a view reads what it selects",
			SQL:   "CREATE VIEW v1 AS SELECT c1 FROM t1",
			Needs: []Right{Manage, mainT1(core.ActionSelect)},
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

		// --- asking the planner about a statement. It reports what the server
		// knows about the rows, so it takes what the statement itself takes.
		{
			Name:  "EXPLAIN of a select",
			SQL:   "EXPLAIN SELECT c1 FROM t1",
			Needs: []Right{mainT1(core.ActionSelect)},
			Why:   "the plan reports what the server knows about the rows of t1",
		},
		{
			Name:  "EXPLAIN of an update",
			SQL:   "EXPLAIN UPDATE t1 SET c1 = 1",
			Needs: []Right{mainT1(core.ActionUpdate)},
			Why:   "asking how a write would run is asking about the write",
		},

		{
			On:    []string{"postgresql", "mysql"},
			Name:  "truncating a table is administration",
			SQL:   "TRUNCATE TABLE t1",
			Needs: []Right{Manage},
			Op:    core.InspectOpTruncate,
			Why:   "it empties the table outside the transaction a delete runs in",
		},
		{
			On:    []string{"sqlite"},
			Name:  "SQLite has no TRUNCATE",
			SQL:   "TRUNCATE TABLE t1",
			Needs: []Right{Manage},
			Why:   "a statement the grammar does not carry takes the administration right",
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
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "an upsert that updates on conflict also updates",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) DO UPDATE SET c2 = 'x'",
			Needs: []Right{mainT1(core.ActionInsert), mainT1(core.ActionUpdate)},
			Op:    core.InspectOpInsert,
			Why:   "a role holding insert alone would rewrite a row it may not update",
		},
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "an upsert that does nothing on conflict only inserts",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) DO NOTHING",
			Needs: []Right{mainT1(core.ActionInsert)},
			Op:    core.InspectOpInsert,
			Why:   "the conflicting row is left exactly as it was",
		},
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "RETURNING hands rows back, which is a read",
			SQL:   "UPDATE t1 SET c1 = 1 RETURNING c2",
			Needs: []Right{mainT1(core.ActionUpdate), mainT1(core.ActionSelect)},
			Op:    core.InspectOpUpdate,
			Why:   "a role holding update alone would read t1 by writing to it",
		},
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "a delete returning what it removed",
			SQL:   "DELETE FROM t1 RETURNING c2",
			Needs: []Right{mainT1(core.ActionDelete), mainT1(core.ActionSelect)},
			Op:    core.InspectOpDelete,
			Why:   "the rows come back to the caller on their way out",
		},
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "an insert returning what it wrote",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) RETURNING c2",
			Needs: []Right{mainT1(core.ActionInsert), mainT1(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "c2 is whatever the table put there, which the caller did not supply",
		},
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "UPDATE ... FROM reads the table it joins against",
			SQL:   "UPDATE t1 SET c1 = 2 FROM t2 WHERE t1.c1 = t2.c1",
			Needs: []Right{mainT1(core.ActionUpdate), mainT2(core.ActionSelect)},
			Op:    core.InspectOpUpdate,
			Why:   "t2 is read to decide which rows of t1 change, and it is not written",
		},
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "an upsert whose conflict predicate reads another table",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) WHERE c1 IN (SELECT c1 FROM t2) DO NOTHING",
			Needs: []Right{mainT1(core.ActionInsert), mainT2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "whether the row is inserted is an answer about t2",
		},
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "an upsert reading another table to update from it",
			SQL:   "INSERT INTO t1 (c1) SELECT c1 FROM t2 ON CONFLICT (c1) DO UPDATE SET c2 = 'x'",
			Needs: []Right{mainT1(core.ActionInsert), mainT1(core.ActionUpdate), mainT2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "it inserts, it rewrites what was there, and it reads t2 to do it",
		},
		{
			On:    []string{"postgresql"},
			Name:  "EXPLAIN of a create from a query",
			SQL:   "EXPLAIN CREATE TABLE t9 AS SELECT c1 FROM t2",
			Needs: []Right{Manage, mainT2(core.ActionSelect)},
			Op:    core.InspectOpCreate,
			Why:   "wrapping a statement in EXPLAIN must not ask for less than the statement does",
		},
		{
			On: []string{"postgresql", "mysql"},
			// EXPLAIN ANALYZE runs the statement rather than describing it,
			// so nothing less than what the statement takes will do.
			Name:  "EXPLAIN ANALYZE of a delete",
			SQL:   "EXPLAIN ANALYZE DELETE FROM t1",
			Needs: []Right{mainT1(core.ActionDelete)},
			Why:   "the rows really go, so it is the delete it explains",
		},
		{
			On:    []string{"mysql", "sqlite"},
			Name:  "REPLACE deletes the row it conflicts with",
			SQL:   "REPLACE INTO t1 (c1) VALUES (1)",
			Needs: []Right{mainT1(core.ActionInsert), mainT1(core.ActionDelete)},
			Op:    core.InspectOpInsert,
			Why:   "the row that was there is gone, so insert alone destroys data",
		},
		{
			On:    []string{"mysql", "sqlite"},
			Name:  "REPLACE from a select reads what it copies",
			SQL:   "REPLACE INTO t1 (c1) SELECT c1 FROM t2",
			Needs: []Right{mainT1(core.ActionInsert), mainT1(core.ActionDelete), mainT2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "it destroys rows of t1 and reads t2 to do it",
		},
		{
			On:    []string{"postgresql", "sqlite"},
			Name:  "a name in double quotes",
			SQL:   `SELECT c1 FROM "t1"`,
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "quoting is how a name is written, not which table it is",
		},
		{
			On:    []string{"postgresql"},
			Name:  "a quoted schema and table",
			SQL:   `SELECT c1 FROM "main"."t1"`,
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "both halves quoted is still main.t1",
		},
		{
			On:    []string{"postgresql"},
			Name:  "copying a file into a table",
			SQL:   "COPY t1 FROM '/tmp/x.csv'",
			Needs: []Right{Manage},
			Why:   "it reads a file the server can see and the caller may not",
		},
		{
			On:    []string{"postgresql"},
			Name:  "copying a table out to a file",
			SQL:   "COPY t1 TO '/tmp/x.csv'",
			Needs: []Right{Manage},
			Why:   "it writes rows somewhere the rules do not reach",
		},
		{
			On:    []string{"mysql", "sqlite"},
			Name:  "a name in backticks",
			SQL:   "SELECT c1 FROM `t1`",
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "backticks quote a name in MySQL, and SQLite takes them too",
		},
		{
			On:    []string{"mysql"},
			Name:  "a backticked schema and table",
			SQL:   "SELECT c1 FROM `main`.`t1`",
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "both halves quoted is still main.t1",
		},
		{
			On:    []string{"mysql"},
			Name:  "calling a procedure",
			SQL:   "CALL some_proc()",
			Needs: []Right{Manage},
			Why:   "what the procedure does is not in the statement",
		},
		{
			On:    []string{"mysql"},
			Name:  "loading a file into a table",
			SQL:   "LOAD DATA INFILE '/tmp/x.csv' INTO TABLE t1",
			Needs: []Right{Manage},
			Why:   "it reads a file the server can see and the caller may not",
		},
		{
			On:    []string{"mysql"},
			Name:  "locking a table",
			SQL:   "LOCK TABLES t1 WRITE",
			Needs: []Right{Manage},
			Why:   "holding a lock is a thing done to the server, not to rows",
		},
		{
			On:    []string{"mysql"},
			Name:  "ON DUPLICATE KEY UPDATE also updates",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON DUPLICATE KEY UPDATE c2 = 'x'",
			Needs: []Right{mainT1(core.ActionInsert), mainT1(core.ActionUpdate)},
			Op:    core.InspectOpInsert,
			Why:   "a role holding insert alone would rewrite a row it may not update",
		},
		{
			On:    []string{"mysql"},
			Name:  "a multi-table UPDATE writes what it sets and reads the rest",
			SQL:   "UPDATE t1 JOIN t2 ON t1.c1 = t2.c1 SET t1.c1 = 2",
			Needs: []Right{mainT1(core.ActionUpdate), mainT2(core.ActionSelect)},
			Op:    core.InspectOpUpdate,
			Why:   "t2 is joined against, so update on it is a right the statement does not need",
		},
		{
			On:    []string{"mysql"},
			Name:  "a multi-table UPDATE writing both tables",
			SQL:   "UPDATE t1 JOIN t2 ON t1.c1 = t2.c1 SET t1.c1 = 2, t2.c3 = 'x'",
			Needs: []Right{mainT1(core.ActionUpdate), mainT2(core.ActionUpdate)},
			Op:    core.InspectOpUpdate,
			Why:   "both are named by the SET list, so both are written",
		},
		{
			On:    []string{"mysql"},
			Name:  "a multi-table DELETE reads what it joins against",
			SQL:   "DELETE t1 FROM t1 JOIN t2 ON t1.c1 = t2.c1",
			Needs: []Right{mainT1(core.ActionDelete), mainT2(core.ActionSelect)},
			Op:    core.InspectOpDelete,
			Why:   "which rows of t1 go is an answer about t2, and only t1 is deleted from",
		},
		{
			On:    []string{"mysql"},
			Name:  "ON DUPLICATE KEY UPDATE reading another table",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON DUPLICATE KEY UPDATE c2 = (SELECT c3 FROM t2)",
			Needs: []Right{mainT1(core.ActionInsert), mainT1(core.ActionUpdate), mainT2(core.ActionSelect)},
			Op:    core.InspectOpInsert,
			Why:   "the value it writes over the old one is read out of t2",
		},
		{
			On:    []string{"sqlite"},
			Name:  "a name in square brackets",
			SQL:   "SELECT c1 FROM [t1]",
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "SQLite takes SQL Server quoting too",
		},
		{
			On:    []string{"sqlite"},
			Name:  "a quoted name in another case",
			SQL:   `SELECT c1 FROM "T1"`,
			Needs: []Right{mainT1(core.ActionSelect)},
			Op:    core.InspectOpSelect,
			Why:   "SQLite matches a table name without regard to case, quoted or not",
		},
		{
			On:    []string{"sqlite"},
			Name:  "attaching another database file",
			SQL:   "ATTACH DATABASE '/tmp/other.db' AS x",
			Needs: []Right{Manage},
			Why:   "it brings a whole database the rules say nothing about into reach",
		},
		{
			On:    []string{"sqlite"},
			Name:  "detaching one",
			SQL:   "DETACH DATABASE x",
			Needs: []Right{Manage},
			Why:   "what the connection can reach is not data",
		},
		{
			On:    []string{"sqlite"},
			Name:  "reading the schema through a pragma",
			SQL:   "PRAGMA table_info(t1)",
			Needs: []Right{Manage},
			Why:   "a pragma reads and writes settings rather than rows",
		},
		{
			On:    []string{"sqlite"},
			Name:  "vacuuming",
			SQL:   "VACUUM",
			Needs: []Right{Manage},
			Why:   "it rewrites the file the database lives in",
		},
	}
}
