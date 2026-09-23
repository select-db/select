---
name: dialect-fixer
description: The standing mission for the dialect package: draft every statement case each dialect can express, state the expected verdict for permissions, completion, linting and resolution, measure what the code actually does, and change the code until the two agree. Use whenever work touches dialect/postgresql, dialect/mysql, dialect/sqlite, dialect/engine or dialect/core/tokenanalyzer, whenever asked to find or fix bugs in SQL language support, harden permission enforcement, investigate a false denial or a missed check, work an issue labelled agent:finder, or check what an inspector, the analyzer or the completion reports for some SQL. Use it even when the request sounds like one narrow question ("does CREATE TABLE AS check permissions?"), because the answer is usually "no, and neither do six other things".
---

# Fixing the dialect package

Read `.claude/dialect/method.md` first: the layers, the defect class, the axes,
the method and the severity ranking live there. `.claude/dialect/probes.md`
carries the probe templates. This skill is what to do with a mismatch: change
the code.

## The mission

Draft every statement case each dialect can express, with the expected verdict,
layer by layer in the method's order. Where the measured verdict does not match
the expectation, change the code until it does. Layer 1 is finished on all
three dialects before layer 2 starts. "Finished" means the case table covers
the statement space for that layer and every case passes on PostgreSQL, MySQL
and SQLite.

Two standing constraints:

**Fix, do not report and stop.** A mismatch between expectation and measurement
is work to do, not a finding to hand over. Stop and ask only where the
expectation itself is a product decision nobody has made, and say which
expectation is in question rather than describing the code.

**Fix generically.** The fourth dialect must inherit the fix without writing it
again. A rule belongs in `core` unless the grammar forces it per dialect; a
case belongs in the shared table unless the SQL is one only some dialects
accept. Where a fix has to be per dialect, write it the same way in all three,
so the pattern is obvious to whoever adds the fourth.

After each fix, rerun the standing must-keep-working list from method step 5,
before and after, and account for every statement that moved.

## Working from finder issues

The `dialect-finder` agent files issues labelled `agent:finder`. Each carries
the repro statements, the expected and measured verdicts, and a ledger finding
id. Treat the expectation as a claim to check against method step 1, not as a
spec: an `oracle:judgment` issue is a question until you have decided it.

Close every one with exactly one verdict label and a one-line reason in the
closing comment. The finder reads both on its next run, and they are its only
feedback:

| Label               | When                                                   |
| ------------------- | ------------------------------------------------------ |
| `verdict:fixed`     | the code changed; link the pull request                |
| `verdict:not-a-bug` | the expectation was wrong; the reason becomes a rule the finder will not file against again, so state it as one ("MySQL REPLACE needing delete is intended") |
| `verdict:duplicate` | name the issue it duplicates                           |
| `verdict:wontfix`   | real, but not worth changing; say why                  |

## Where the cases live

The see work built the shape the other layers follow.

- **`core/testutil/see_data.go`** holds the case type and the tables: one
  shared table every dialect runs, plus a table for SQL that only some accept
  (`GetSeeCasesPostgreSQLAndSQLite`).
- **`core/testutil/see.go`** holds the runner. A dialect's entry point is three
  lines, so a fourth dialect inherits the whole table by calling it.
- **`dialect/<dialect>/see_test.go`** holds only what that dialect alone
  expresses: its quoting, its own statement forms.

Build each new layer the same way: a case type that states the expectation, one
runner in `core/testutil`, shared tables for what every dialect expresses, a
per-dialect table for the rest. A case that two dialects accept goes in a table
those two share rather than a copy each -- two copies of a case diverge, and
the FILTER case proved it.

`core/testutil` may import `core` but not `engine`, which is what lets the
engine's own tests use it.

Reuse `core.GetInspectTestMetadata()` or `testutil.GetSeeTestMetadata()` for
the catalog rather than building a fixture.

## Writing a case that proves something

**Necessary and sufficient.** A permission case that only asserts "refused"
passes just as well when everything is refused. Assert both directions:

```go
for idx, missing := range tt.needs {
    rest := slices.Delete(slices.Clone(tt.needs), idx, idx+1)
    if err := core.CheckQueryPermissions(inspected, dbID, holding(rest...)); err == nil {
        t.Errorf("ran without %q. %s", missing, tt.why)
    }
}
if err := core.CheckQueryPermissions(inspected, dbID, holding(tt.needs...)); err != nil {
    t.Errorf("holding %v still refused it: %v", tt.needs, err)
}
```

**Guard against a vacuous pass.** If a case resolves no table, the check has
nothing to refuse and goes green. If a case expects masking but names no result
columns, there is nothing to mask. Make the runner fail such a case rather than
report it:

```go
if len(stmt.Tables) == 0 {
    t.Fatalf("resolved no table, so the check had nothing to refuse: %+v", stmt)
}
```

**Verify the case fails against the old code.** A test that passes before the
fix tests nothing.

```sh
cd dialect
git stash push -- core/resolve.go postgresql/inspect.go mysql/inspect.go sqlite/inspect.go
go test ./... -run TestYourNewTest    # must fail, and count the failures
git stash pop
```

State the count in the commit message. "13 of 16 cases fail against the
pre-fix inspectors" is a claim a reviewer can check in thirty seconds.

## House rules that bite here

The root `CLAUDE.md` and `CONTRIBUTING.md` carry the repository's rules, and
they win. Two have a specific shape in this package:

- Compose what exists before adding a layer. `core.Resolver`, `core.Scope`,
  `core.OrUnknown` and `core.UnknownStatement` already exist; `engine.Inspect`
  already floors an unrecognised statement. Reach for those before inventing a
  guard.
- Never hand-edit the generated parser files under `*/parser/`. A grammar fix
  is a regeneration, not an edit.

## Cleanup

Probes are scratch. Delete them before staging anything, and stage explicit
paths rather than `git add -A`. Verify with `git status --short` and
`find . -name 'zz_*'` before committing.

## Checks before pushing

```sh
cd dialect
golangci-lint run ./...          # CONTRIBUTING.md: CI also fails on dead code
SELECT_REQUIRE_ANALYZER=1 go test ./...
gofmt -l <the files you touched>
go run ./cmd/seesweep            # 0 leaks, 0 oracles
```

`SELECT_REQUIRE_ANALYZER=1` makes the Python analyzer tests fail loudly instead
of skipping, which is the difference between "the suite is green" and "the
suite ran".

`golangci-lint` may refuse to run locally when its binary is built against an
older Go than the module targets. `go run
github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 run ./...` runs
the version CI pins. Dead code is what it caught the last two times, so re-read
the diff for a helper that lost its last caller before pushing.
