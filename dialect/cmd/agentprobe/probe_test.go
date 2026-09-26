package main

import (
	"slices"
	"testing"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

func TestMeasurePermissions(t *testing.T) {
	meta := core.GetInspectTestMetadata()
	cases := []struct {
		name, dialect, sql string
		needs              []string
		allowed            []string // policies that must run it; the rest must refuse
	}{
		{
			name:    "a plain read needs select on each column it returns",
			dialect: "postgresql",
			sql:     "SELECT c1 FROM t1",
			needs:   []string{"select on main.t1.c1"},
			allowed: []string{"data"},
		},
		{
			name:    "a write inside a CTE is asked for alongside the read around it",
			dialect: "postgresql",
			sql:     "WITH x AS (DELETE FROM t1 RETURNING c1) SELECT c1 FROM x",
			needs:   []string{"select on main.t1.c1", "delete on main.t1"},
			allowed: []string{"data"},
		},
		{
			name:    "manage refused on a named object is granted on the connection",
			dialect: "postgresql",
			sql:     "CREATE VIEW v9 AS SELECT c1 FROM t1",
			needs:   []string{"manage", "select on main.t1.c1"},
			allowed: nil,
		},
		{
			name:    "a statement nothing recognised takes manage",
			dialect: "sqlite",
			sql:     "VACUUM",
			needs:   []string{"manage"},
			allowed: []string{"manage"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			statements := engine.Inspect(engine.GetDialect(tc.dialect), &meta, tc.sql)
			measured := measurePermissions(statements)
			if !measured.Converged {
				t.Fatalf("never converged, holding %v", measured.Needs)
			}
			if !slices.Equal(measured.Needs, tc.needs) {
				t.Errorf("needs %v, want %v", measured.Needs, tc.needs)
			}
			for _, verdict := range measured.Policies {
				if want := slices.Contains(tc.allowed, verdict.Policy); verdict.Allowed != want {
					t.Errorf("%s allowed=%t, want %t (%s)", verdict.Policy, verdict.Allowed, want, verdict.Denial)
				}
			}
		})
	}
}
