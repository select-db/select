# Probe templates

A probe asks the real code what it does. Write it, read the output, delete it.

Run everything from `dialect/`. That module is separate from `app/`, so `go
test` at the repository root does not find it.

## One statement, every layer: sqlprobe

`dialect/cmd/sqlprobe` already runs lint, completion and `Inspect` over one
statement and prints each result. Reach for it before writing anything.

`-meta` is a JSON file unmarshalled straight into `core.Metadata`, so the keys
are the Go field names. This one is enough to probe with:

```json
{
  "DefaultDB": "db1",
  "DefaultSchema": "main",
  "Schemas": [
    {
      "Name": "main",
      "Tables": [
        {"Name": "t1", "Columns": [{"Name": "c1", "Type": "integer"},
                                   {"Name": "c2", "Type": "text"}]}
      ]
    }
  ]
}
```

```sh
cd dialect
go run ./cmd/sqlprobe -dialect postgresql -meta /tmp/meta.json \
    -sql 'SELECT c1 FROM t1 WHERE |'
```

`|` marks the caret for completion, `@file` reads the SQL from a file, and
`-raw` adds the analyzer's own view: the relations, virtual tables, column refs
and completion context it resolved. It needs the Python venv: `uv sync` in
`dialect/core/tokenanalyzer/python`.

It does not decide permissions, and it takes one statement at a time. For those
two cases, write a Go probe.

## Permissions: what does this statement need?

`dialect/engine/zz_probe_test.go`. Name it `zz_` so it sorts last and is
obvious, and delete it before staging. The `compileFor` helper already exists
in `engine/execute_see_test.go`.

```go
package engine

import (
	"fmt"
	"testing"

	"github.com/selectDb/dialect/core"
)

func TestZZProbe(t *testing.T) {
	meta := core.GetInspectTestMetadata() // schema "main": t1(c1,c2), t2(c1,c3), other.t3

	mk := func(actions ...string) core.CompiledPermissions {
		var e []core.PermissionEntry
		for _, a := range actions {
			e = append(e, core.PermissionEntry{Action: a, Effect: "allow", RoleName: "r"})
		}
		return compileFor("db1", e...).WithDenyUnmanaged()
	}
	none := core.Compile(nil).WithDenyUnmanaged()
	analyst := mk(core.ActionSelect, core.ActionInsert, core.ActionUpdate, core.ActionDelete)
	writer := mk(core.ActionInsert, core.ActionUpdate, core.ActionDelete) // no select
	admin := mk(core.ActionManage)

	type probe struct{ dialect, sql string }
	for _, p := range []probe{
		{"postgresql", "WITH x AS (DELETE FROM t1 RETURNING c1) SELECT c1 FROM x"},
		{"postgresql", "SELECT c1 FROM t1"},
	} {
		stmts := Inspect(GetDialect(p.dialect), &meta, p.sql)
		shape := ""
		for _, s := range stmts {
			shape += fmt.Sprintf("op=%s tables=%d subs=%d", s.Operation, len(s.Tables), len(s.Subqueries))
		}
		v := func(perms core.CompiledPermissions) string {
			if core.CheckQueryPermissions(stmts, "db1", perms) == nil {
				return "ALLOWED"
			}
			return "denied"
		}
		t.Logf("%-10s none=%-8s writer=%-8s analyst=%-8s manage=%-8s %-26s %s",
			p.dialect, v(none), v(writer), v(analyst), v(admin), shape, p.sql)
	}
}
```

Run it with `-v` to see the log lines. `none=ALLOWED` is a total bypass.
`writer=ALLOWED` or `manage=ALLOWED` on a statement that reads means the read
is unchecked. `tables=0` next to either is usually the cause: a check over no
table has nothing to refuse.

`engine.Inspect` floors an unrecognised top-level statement to one unknown
statement, so a probe through it never reports none. Zero statements is a
`dialect.Inspect` shape, and a hole only for a dialect registered from outside
this module.

## Inspect shape: what did the parser make of this?

When you need the tree rather than the verdict, recurse:

```go
func dump(t *testing.T, indent string, stmts []core.InspectStatement) {
	for _, s := range stmts {
		names := ""
		for _, tb := range s.Tables {
			names += fmt.Sprintf(" %s.%s", tb.Schema, tb.Name)
		}
		t.Logf("%s op=%-8s tables=[%s] fields=%d subs=%d",
			indent, s.Operation, names, len(s.Fields), len(s.Subqueries))
		dump(t, indent+"    ", s.Subqueries)
	}
}
```

A table with `Schema: ""` did not resolve in the metadata, and the checker
denies it. That is the signature of the false denial the method's step 4 looks
for: a CTE or subquery alias read as a real table.

## Does the database accept this at all?

Before treating a misclassification as a hole, check the statement is runnable.
SQLite is in the module already:

```go
import (
	"database/sql"
	_ "modernc.org/sqlite"
)

db, _ := sql.Open("sqlite", ":memory:")
db.Exec("CREATE TABLE t1 (c1 INTEGER)")
_, err := db.Query("(SELECT c1 FROM t1)")   // syntax error here
```

Reachability is per dialect, and the answer decides the fix, not whether there
is one. That same parenthesized query and the `TABLE t1` shorthand are both
total bypasses on PostgreSQL; SQLite refuses to parse either, so its inspector
needed no change. Ask each server rather than generalising from one.

For PostgreSQL or MySQL, start a real server and execute it. A classification
every server rejects is not a bypass, and saying so beats writing code for an
unreachable case.

## Lint and diagnostics

The analyzer is a Python subprocess. Its tests live in
`dialect/core/tokenanalyzer/python/` and go through
`analyze(sql, dialect=..., schema_dict=..., default_schema=...)`, whose
`diagnostics` key is what a lint finding comes back in. The suites there
already wrap it; copy the wrapper from the rule file you are working on rather
than writing a third one.

`conftest.py` sets `SELECT_STRICT_DIAGNOSTICS=1` for the whole suite, so a rule
that cannot place its own range raises instead of silently widening. Keep it
on: a diagnostic an editor cannot draw reads to a user exactly like no
diagnostic at all, and a test counting rule ids cannot tell them apart.

To check what a diagnostic covers, slice the source with it:

```python
def marked(sql, d):
    lines = sql.splitlines()
    first, last = d["start_line"] - 1, d["end_line"] - 1
    if first == last:
        return lines[first][d["start_col"]:d["end_col"]]
    return "\n".join([lines[first][d["start_col"]:]]
                     + lines[first + 1:last]
                     + [lines[last][:d["end_col"]]])
```

## Counting work rather than timing it

For a latency question, count the operation rather than measuring the clock:
the count is stable across machines and names the cause. Monkeypatch the thing
you suspect and assert the count does not grow with input size.

`_scans_during` in `lint_rules/file_level_test.py` does exactly this to
`Dialect.tokenize`, and asserts one statement costs the same number of scans as
twenty. Copy that, pointed at whatever you suspect.

## Hover and references

Neither has a probe here, and neither lives in this module. Hover is
`app/internal/sqllang/hover.go`, tested next to it, and it reads the catalog
and the reference resolution this package produces. References come out of
`core/references`; `sqlprobe -raw` prints what the analyzer resolved for a
statement, which is the fastest way to tell a hover bug from the reference
resolution under it.

## Operators and types

To check what the completion offers for a column type, execute each operator
against a real server rather than reading `pg_operator`. Range operators are
defined over `anyrange` and do not appear under the concrete type name, so a
catalog lookup reports them missing when they work. Build one probe statement
per operator, run it, and record whether the server accepted it.
