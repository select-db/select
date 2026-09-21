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
// Cases here MUST hold for every dialect that accepts the SQL. A statement
// only some dialects parse goes in the table for those dialects.
type PermCase struct {
	Name  string
	SQL   string
	Needs []string
	Why   string
}

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
			Needs: []string{core.ActionSelect},
			Why:   "it returns rows of t1",
		},
		{
			Name:  "an insert writes",
			SQL:   "INSERT INTO t1 (c1) VALUES (1)",
			Needs: []string{core.ActionInsert},
			Why:   "it adds a row and reads nothing",
		},
		{
			Name:  "an update writes",
			SQL:   "UPDATE t1 SET c1 = 1",
			Needs: []string{core.ActionUpdate},
			Why:   "it changes rows and returns none",
		},
		{
			Name:  "a delete writes",
			SQL:   "DELETE FROM t1",
			Needs: []string{core.ActionDelete},
			Why:   "it removes rows and returns none",
		},

		// --- a write carrying a read
		{
			Name:  "an insert from a select reads what it copies",
			SQL:   "INSERT INTO t1 (c1) SELECT c1 FROM t2",
			Needs: []string{core.ActionInsert, core.ActionSelect},
			Why:   "holding insert alone would copy t2 into a table the role can read",
		},
		{
			Name:  "an update from a subquery reads it",
			SQL:   "UPDATE t1 SET c1 = (SELECT c1 FROM t2)",
			Needs: []string{core.ActionUpdate, core.ActionSelect},
			Why:   "the value written is read out of t2",
		},
		{
			Name:  "a delete filtered by a subquery reads it",
			SQL:   "DELETE FROM t1 WHERE c1 IN (SELECT c1 FROM t2)",
			Needs: []string{core.ActionDelete, core.ActionSelect},
			Why:   "which rows go depends on what t2 holds",
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
			Needs: []string{core.ActionManage},
			Why:   "the schema is not data",
		},
		{
			Name:  "creating a table from a query reads the query",
			SQL:   "CREATE TABLE t9 AS SELECT c1 FROM t1",
			Needs: []string{core.ActionManage, core.ActionSelect},
			Why:   "manage covers making the table, not reading t1 into it",
		},
		{
			Name:  "creating a view reads what it selects",
			SQL:   "CREATE VIEW v1 AS SELECT c1 FROM t1",
			Needs: []string{core.ActionManage, core.ActionSelect},
			Why:   "the view body is a read that anyone selecting the view inherits",
		},
		{
			Name:  "altering a table is administration",
			SQL:   "ALTER TABLE t1 ADD COLUMN c9 INTEGER",
			Needs: []string{core.ActionManage},
			Why:   "it changes the schema",
		},
		{
			Name:  "dropping a table is administration",
			SQL:   "DROP TABLE t1",
			Needs: []string{core.ActionManage},
			Why:   "delete removes rows, drop removes the table",
		},

		// --- rights and session statements
		{
			Name:  "granting a right is administration",
			SQL:   "GRANT SELECT ON t1 TO bob",
			Needs: []string{core.ActionManage},
			Why:   "it hands a right to somebody else",
		},
		{
			Name:  "revoking a right is administration",
			SQL:   "REVOKE SELECT ON t1 FROM bob",
			Needs: []string{core.ActionManage},
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
			Needs: []string{core.ActionInsert, core.ActionUpdate},
			Why:   "a role holding insert alone would rewrite a row it may not update",
		},
		{
			Name:  "an upsert that does nothing on conflict only inserts",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) DO NOTHING",
			Needs: []string{core.ActionInsert},
			Why:   "the conflicting row is left exactly as it was",
		},
		{
			Name:  "RETURNING hands rows back, which is a read",
			SQL:   "UPDATE t1 SET c1 = 1 RETURNING c2",
			Needs: []string{core.ActionUpdate, core.ActionSelect},
			Why:   "a role holding update alone would read t1 by writing to it",
		},
		{
			Name:  "a delete returning what it removed",
			SQL:   "DELETE FROM t1 RETURNING c2",
			Needs: []string{core.ActionDelete, core.ActionSelect},
			Why:   "the rows come back to the caller on their way out",
		},
		{
			Name:  "an insert returning what it wrote",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) RETURNING c2",
			Needs: []string{core.ActionInsert, core.ActionSelect},
			Why:   "c2 is whatever the table put there, which the caller did not supply",
		},
		{
			Name:  "an upsert reading another table to update from it",
			SQL:   "INSERT INTO t1 (c1) SELECT c1 FROM t2 ON CONFLICT (c1) DO UPDATE SET c2 = 'x'",
			Needs: []string{core.ActionInsert, core.ActionUpdate, core.ActionSelect},
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
			Needs: []string{core.ActionInsert, core.ActionDelete},
			Why:   "the row that was there is gone, so insert alone destroys data",
		},
		{
			Name:  "REPLACE from a select reads what it copies",
			SQL:   "REPLACE INTO t1 (c1) SELECT c1 FROM t2",
			Needs: []string{core.ActionInsert, core.ActionDelete, core.ActionSelect},
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
			Needs: []string{core.ActionInsert, core.ActionUpdate},
			Why:   "a role holding insert alone would rewrite a row it may not update",
		},
		{
			Name:  "ON DUPLICATE KEY UPDATE reading another table",
			SQL:   "INSERT INTO t1 (c1) VALUES (1) ON DUPLICATE KEY UPDATE c2 = (SELECT c3 FROM t2)",
			Needs: []string{core.ActionInsert, core.ActionUpdate, core.ActionSelect},
			Why:   "the value it writes over the old one is read out of t2",
		},
	}
}
