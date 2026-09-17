package engine

import (
	"strings"
	"testing"

	"github.com/selectDb/dialect/core"
)

// permMeta is a catalog the inspectors can resolve names against, in the
// spelling each dialect uses for its default schema.
func permMeta(defaultSchema string) *core.Metadata {
	return &core.Metadata{
		DefaultSchema: defaultSchema,
		Schemas: []core.Schema{{
			Name: defaultSchema,
			Tables: []core.Table{{
				Name:    "users",
				Columns: []core.Column{{Name: "id", Type: "integer"}, {Name: "bio", Type: "text"}},
			}, {
				Name:    "orders",
				Columns: []core.Column{{Name: "id", Type: "integer"}, {Name: "user_id", Type: "integer"}},
			}},
		}},
	}
}

// everyDataAction allows the four actions a statement can be resolved down to,
// and nothing else. A statement that passes under it is one we understood.
func everyDataAction(dbID string) core.CompiledPermissions {
	star := "*"
	var entries []core.PermissionEntry
	for _, a := range []string{core.ActionSelect, core.ActionInsert, core.ActionUpdate, core.ActionDelete} {
		entries = append(entries, core.PermissionEntry{
			DbInstanceID: &dbID, SchemaName: &star, TableName: &star, ColumnName: &star,
			Action: a, Effect: "allow", RoleName: "analyst",
		})
	}
	return core.Compile(entries)
}

const permDBID = "inst-1"

// TestPermissions_StatementsThatNeedManage pins the statements that used to run
// unchecked. Each one inspected to nothing, and a check that iterates statements
// reads nothing as nothing to check, so the whole set executed under a policy
// that grants no manage.
func TestPermissions_StatementsThatNeedManage(t *testing.T) {
	tests := []struct {
		dialect       string
		defaultSchema string
		sql           []string
	}{
		{
			dialect:       "postgresql",
			defaultSchema: "public",
			sql: []string{
				"COPY users FROM '/tmp/x.csv'",
				"COPY users TO '/tmp/x.csv'",
				"CREATE TABLE t2 AS SELECT * FROM users",
				"DO $$ BEGIN DELETE FROM users; END $$",
				"MERGE INTO users u USING orders o ON u.id = o.user_id WHEN MATCHED THEN UPDATE SET bio = 'x'",
				"GRANT SELECT ON users TO bob",
				"REVOKE ALL ON users FROM bob",
				"CREATE ROLE evil SUPERUSER",
				"CREATE VIEW v AS SELECT * FROM users",
				"EXPLAIN ANALYZE DELETE FROM users",
				"ALTER TABLE users RENAME TO users2",
				"REFRESH MATERIALIZED VIEW mv",
				"LOCK TABLE users",
				"CALL some_proc()",
				"DROP TABLE users",
				"TRUNCATE users",
				"!!! not sql at all !!!",
			},
		},
		{
			dialect:       "mysql",
			defaultSchema: "shop",
			sql: []string{
				"LOAD DATA INFILE '/tmp/x' INTO TABLE users",
				"GRANT SELECT ON users TO bob",
				"RENAME TABLE users TO users2",
				"CALL p()",
				"DROP TABLE users",
			},
		},
		{
			dialect:       "sqlite",
			defaultSchema: "main",
			sql: []string{
				"ATTACH DATABASE '/tmp/evil.db' AS e",
				"DROP TABLE users",
				"ALTER TABLE users RENAME TO users2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			d := GetDialect(tt.dialect)
			if d == nil {
				t.Fatalf("no dialect %q", tt.dialect)
			}
			meta := permMeta(tt.defaultSchema)
			perms := everyDataAction(permDBID)

			for _, sql := range tt.sql {
				t.Run(sql, func(t *testing.T) {
					inspected := Inspect(d, meta, sql)
					if len(inspected) == 0 {
						t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
					}
					err := core.CheckQueryPermissions(inspected, permDBID, perms)
					if err == nil {
						t.Fatal("allowed without manage")
					}
					if !strings.Contains(err.Error(), "manage") {
						t.Errorf("denied for the wrong reason: %v", err)
					}
				})
			}
		})
	}
}

// TestPermissions_DataStatementsAreUnaffected pins the other side: the four
// operations we resolve down to columns still pass on their own action, so
// routing the rest through manage did not turn ordinary work into a denial.
func TestPermissions_DataStatementsAreUnaffected(t *testing.T) {
	for _, dialect := range []string{"postgresql", "mysql", "sqlite"} {
		t.Run(dialect, func(t *testing.T) {
			defaultSchema := map[string]string{"postgresql": "public", "mysql": "shop", "sqlite": "main"}[dialect]
			d := GetDialect(dialect)
			meta := permMeta(defaultSchema)
			perms := everyDataAction(permDBID)

			for _, sql := range []string{
				"SELECT id FROM users",
				"SELECT u.id FROM users u JOIN orders o ON u.id = o.user_id",
				"INSERT INTO users (id) VALUES (1)",
				"UPDATE users SET bio = 'x' WHERE id = 1",
				"DELETE FROM users WHERE id = 1",
			} {
				t.Run(sql, func(t *testing.T) {
					inspected := Inspect(d, meta, sql)
					if err := core.CheckQueryPermissions(inspected, permDBID, perms); err != nil {
						t.Errorf("want allowed, got %v", err)
					}
				})
			}
		})
	}
}

// Blank input is not a statement, so it stays empty rather than becoming an
// unknown one that a caller would then have to refuse.
func TestInspect_BlankSQLStaysEmpty(t *testing.T) {
	for _, dialect := range []string{"postgresql", "mysql", "sqlite"} {
		t.Run(dialect, func(t *testing.T) {
			defaultSchema := map[string]string{"postgresql": "public", "mysql": "shop", "sqlite": "main"}[dialect]
			for _, sql := range []string{"", "   ", "\n\t "} {
				if got := Inspect(GetDialect(dialect), permMeta(defaultSchema), sql); len(got) != 0 {
					t.Errorf("Inspect(%q) = %d statements, want 0", sql, len(got))
				}
			}
		})
	}
}
