---
name: dialect-audit
description: The standing mission for the dialect package: draft every statement case each dialect can express, state the expected verdict for permissions, completion, linting and resolution, measure what the code actually does, and change the code until the two agree. Use whenever work touches dialect/postgresql, dialect/mysql, dialect/sqlite, dialect/engine or dialect/core/tokenanalyzer, whenever asked to find or fix bugs in SQL language support, harden permission enforcement, investigate a false denial or a missed check, or check what an inspector, the analyzer or the completion reports for some SQL. Use it even when the request sounds like one narrow question ("does CREATE TABLE AS check permissions?"), because the answer is usually "no, and neither do six other things".
---

# The dialect package

This package decides what a query is allowed to do, what is wrong with it, and
what to suggest next. It has two parsers: ANTLR inspectors in Go (`Inspect()`,
feeding permissions) and sqlglot in a Python subprocess (lint, completion,
references).

## The mission

Draft every statement case each dialect can express, with the expected verdict,
for each of these layers **in this order**:

1. **Permissions** -- which rights a statement requires.
2. **Completion** -- what is suggested at a position.
3. **Linter** -- which diagnostics a statement raises, and where.
4. **Resolution** -- hover text, references, go-to-definition.

Where the measured verdict does not match the expectation, change the code
until it does. Layer 1 is finished on all three dialects before layer 2 starts.
"Finished" means the case table covers the statement space for that layer and
every case passes on PostgreSQL, MySQL and SQLite.

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

## The defect class this package has

**This layer degrades to "nothing here" rather than failing, and "nothing here"
is indistinguishable from a correct empty result.**

A branch the inspector cannot read becomes zero tables. A nested statement it
does not dispatch on becomes no statement at all. A rule that cannot place its
diagnostic becomes a zero-width range at the origin. A consumer that iterates
and finds nothing concludes there is nothing to do, so the query runs
unchecked, the file lints clean, the user sees no marker.

`engine.Inspect` floors a whole statement that comes back unrecognised to
`core.UnknownStatement()`, which takes manage. The floor is per top-level
statement, so everything below it (a table list, a CTE body, a source query)
still degrades silently. Probe through `engine.Inspect`, not `dialect.Inspect`,
or you measure a hole the seam already closes.

Hold this as a prior, not a conclusion. Confirm it by measurement.

## Enumerating the statement space

"All possible cases" is only checkable if the enumeration has a shape. Walk the
axes and take each cell; a cell with no case is a gap, and a gap is the thing
this mission exists to remove.

### Axes for permissions

- **Operation**: select, insert, update, delete, create, alter, drop, truncate,
  grant, revoke, and what one dialect alone has (COPY, REPLACE, upsert, ATTACH,
  PRAGMA, VACUUM, CALL, SET, EXPLAIN, ANALYZE, multi-table UPDATE and DELETE).
- **Where a read hides**: the FROM list, a join, a CTE body, a derived table, a
  scalar subquery in the select list, a subquery in WHERE, GROUP BY, HAVING,
  ORDER BY or LIMIT, a RETURNING clause, an upsert predicate, the source of an
  INSERT ... SELECT or a CREATE TABLE AS, a view body, each branch of a set
  operation.
- **How a relation is named**: bare, schema-qualified, aliased, quoted in each
  way the dialect accepts, in another case, shadowing a CTE, shadowed by one.
- **What the statement does with the table**: returns it, writes it, tests it
  without returning it (which is the see boundary), or names it without
  reading it.
- **Policy**: deny-all, the four data actions without manage, per-table grants,
  manage alone.

### Axes for the later layers

Completion: the position in the statement (after SELECT, after FROM, after a
dot, inside a function call, in a WHERE, mid-identifier), what is in scope at
it (tables, CTEs, aliases, columns of each), and what the user has typed so
far. Linting: one case per rule, times the ways a statement can make the rule
fire, plus the range the diagnostic must carry. Resolution: one case per
identifier kind, at each position it can appear.

## Method

### 1. Write the expectation first

The case states the verdict before anything is measured. For permissions that
is the exact set of rights: `select on main.users`, `manage`, and so on. An
expectation written after reading the output is a transcript, not a test.

Where the measurement disagrees, decide which side is wrong on the semantics of
SQL and of the permission model, not on which is easier to change. Both
outcomes happen: three expectations in the see work were wrong and the code was
right.

### 2. Probe before you read the source

Do not reason about an inspector from its source. Ask it.

`dialect/cmd/sqlprobe` is the first reach: one statement, every layer it
covers, enough to isolate which one is wrong. `dialect/cmd/seesweep` generates
statements and runs them through the engine against a twin database, which is
how a case nobody thought to write gets found. Where neither reaches, write a
throwaway Go or Python probe; `references/probes.md` carries the templates.

The parse tree and the server are both more surprising than the source reads,
so a prediction made from the source is a hypothesis, not a finding.

### 3. Measure the verdict, not the shape

An intermediate structure is a clue. The decision function is the finding.

```
op=select tables=0 subs=0                <- a clue
ALLOWED under a policy granting nothing  <- a finding
```

Always run the probe's output through the thing that decides:
`core.CheckQueryPermissions` and `core.CheckSeePredicates` for permissions,
`analyze()` for lint, the completion entry point for suggestions.

### 4. Bracket with two policies

For permissions, run every candidate under both:

- **deny-all**: `core.Compile(nil).WithDenyUnmanaged()`. Anything that passes
  here passes every policy that exists. That is a total bypass.
- **the four data actions**: select, insert, update, delete on every table, no
  manage. Anything that passes here is a management action available without
  the administration right.

The gap between them is where the findings are.

### 5. Probe the opposite direction too

Every tightening risks a false denial, and a false denial is a support ticket
from a user who did nothing wrong. After each fix, run ordinary work through a
realistic role and confirm it still passes.

Keep a standing list of statements that must keep working: `SELECT 1` (resolves
no table legitimately, on every dialect), a plain `UPDATE`, a CTE, a FROM
subquery, a join with an alias. Reporting a new read is the shape that breaks
these: a name the new code path does not know is virtual resolves to no schema,
and the checker then denies it for every role.

Measure it rather than asserting it. Run the corpus before and after, diff the
set of statements that ran, and account for every statement that moved.

### 6. Ask the real server when the question is about the server

When the question is "what does the database do", start one and ask. Do not
infer from catalog names or documentation.

Executing every operator the completion offers against a real PostgreSQL found
30 it refuses, `point = point` and `json = json` among them. An in-memory
SQLite settles whether a statement parses at all, which decides whether a
misclassification is even reachable.

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

## Reporting

Group by **service** (permissions, linting, completion, hovering, references)
and by **cause** (the ANTLR inspectors, the sqlglot analyzer, metadata
resolution, or the consumer in `core/permissions.go`), then order by severity:

- a **bypass** (ran with no grant) outranks
- a **wrong right** (ran under the wrong permission) outranks
- an **unchecked read** (the write was checked, the read inside was not) outranks
- a **false denial** (fail-closed, annoying, not dangerous).

Split work by **kind of resolution**, not by symptom. Three bugs that all come
from "a nested statement vanished" are one change. A fourth from "a write is
classified as a read" is a different change even in the same file.

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
older Go than the module targets. It still runs in CI, and dead code is what it
caught the last two times, so re-read the diff for a helper that lost its last
caller before pushing.
