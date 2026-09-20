---
name: dialect-audit
description: Audit a layer of the dialect package (permissions, linting, completion, hovering, operators) by probing real SQL through the real code path and measuring what it does, rather than reading the source and reasoning about it. Use this whenever you are asked to find bugs in the SQL language support, harden permission enforcement, check what an inspector or the analyzer reports for some SQL, investigate a false denial or a missed check, or whenever work touches dialect/postgresql, dialect/mysql, dialect/sqlite, dialect/engine or dialect/core/tokenanalyzer. Use it even when the request sounds like a single narrow question ("does CREATE TABLE AS check permissions?"), because the method's whole value is that the answer is usually "no, and neither do six other things".
---

# Auditing the dialect package

This package decides what a query is allowed to do, what is wrong with it, and
what to suggest next. It has two parsers: ANTLR inspectors in Go (`Inspect()`,
feeding permissions) and sqlglot in a Python subprocess (lint, completion,
references). Both are large and both fail the same way.

## The defect class this package has

**This layer degrades to "nothing here" rather than failing, and "nothing here"
is indistinguishable from a correct empty result.**

A branch the inspector cannot read becomes zero tables. A nested statement it
does not dispatch on becomes no statement at all. A rule that cannot place its
diagnostic becomes a zero-width range at the origin. A consumer that iterates
and finds nothing concludes there is nothing to do, so the query runs
unchecked, the file lints clean, the user sees no marker. Every audit worth
doing starts by looking for a place where absence is being read as correctness.

One case is already closed: `engine.Inspect` floors a whole statement that
comes back unrecognised to `core.UnknownStatement()`, which takes manage. The
floor is per top-level statement, so everything below it (a table list, a CTE
body, a source query) still degrades silently. Probe through `engine.Inspect`,
not `dialect.Inspect`, or you measure a hole the seam already closes.

Hold this as a prior, not a conclusion. Confirm it by measurement.

## Method

### 1. Probe before you read

Do not reason about an inspector from its source. Ask it.

`dialect/cmd/sqlprobe` is the first reach: one statement, every layer it
covers, enough to isolate which one is wrong. Where it does not reach, write a
throwaway Go or Python probe. `references/probes.md` says what each covers and
carries the templates.

The parse tree and the server are both more surprising than the source reads,
so a prediction made from the source is a hypothesis, not a finding.

### 2. Measure the verdict, not the shape

An intermediate structure is a clue. The decision function is the finding.

```
op=select tables=0 subs=0                <- a clue
ALLOWED under a policy granting nothing  <- a finding
```

Always run the probe's output through the thing that decides:
`core.CheckQueryPermissions` for permissions, `analyze()` for lint, the
completion entry point for suggestions. A reviewer can argue with a shape. They
cannot argue with "this statement ran and nobody was asked".

### 3. Bracket with two policies

For permissions, run every candidate under both:

- **deny-all**: `core.Compile(nil).WithDenyUnmanaged()`. Anything that passes
  here passes every policy that exists. That is a total bypass.
- **the four data actions**: select, insert, update, delete on every table, no
  manage. Anything that passes here is a management action available without
  the administration right.

The gap between them is where the findings are.

### 4. Probe the opposite direction too

Every tightening risks a false denial, and a false denial is a support ticket
from a user who did nothing wrong. After each fix, run ordinary work through a
realistic role and confirm it still passes.

Keep a standing list of statements that must keep working: `SELECT 1` (resolves
no table legitimately, on every dialect), a plain `UPDATE`, a CTE, a FROM
subquery, a join with an alias. Reporting a new read is the shape that breaks
these: a name the new code path does not know is virtual resolves to no schema,
and the checker then denies it for every role.

### 5. Ask the real server when the question is about the server

When the question is "what does the database do", start one and ask. Do not
infer from catalog names or documentation.

Executing every operator the completion offers against a real PostgreSQL found
30 it refuses, `point = point` and `json = json` among them. An in-memory
SQLite settles whether a statement parses at all, which decides whether a
misclassification is even reachable. Both take minutes and both change the
design.

## Classify before proposing

Findings are prioritised by service and by cause, so present them grouped both
ways.

**By service**: permissions, linting, completion, hovering, references.

**By cause**: the ANTLR inspectors (`dialect/*/inspect.go`), the sqlglot
analyzer (`dialect/core/tokenanalyzer/python/`), metadata resolution, or the
consumer itself (`dialect/core/permissions.go`).

Then order by severity, and be honest about what each one costs:

- a **bypass** (ran with no grant) outranks
- a **wrong right** (ran under the wrong permission) outranks
- an **unchecked read** (the write was checked, the read inside was not) outranks
- a **false denial** (fail-closed, annoying, not dangerous).

## Present findings before writing code

Report what you measured and stop. The owner decides what to take and in what
order. An audit usually turns up more than one PR's worth of work, and the
split between "fix now" and "different mechanism, different PR" is a judgement
about their roadmap.

When you propose the split, group by **kind of resolution**, not by symptom.
Three bugs that all come from "a nested statement vanished" are one PR. A
fourth that comes from "a write is classified as a read" is a different PR even
though it sits in the same file.

## Regression tests

A test that passes against the old code tests nothing. Verify it:

```sh
cd dialect
git stash push -- postgresql/inspect.go mysql/inspect.go sqlite/inspect.go
go test ./engine/ -run TestYourNewTest    # must fail, and count the failures
git stash pop
```

State the count in the PR. "13 of 16 cases fail against the pre-fix inspectors"
is a claim a reviewer can check in thirty seconds.

**Necessary and sufficient.** A permission test that only asserts "refused"
passes just as well when everything is refused. Assert both directions: holding
every permission but one must be refused, and holding all of them must run.

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

**Guard against a vacuous pass.** If a test resolves no table, the check has
nothing to refuse and goes green. Assert the fixture resolved:

```go
if len(stmt.Tables) == 0 {
    t.Fatalf("resolved no table, so the check had nothing to refuse: %+v", stmt)
}
```

**Where tests go.** One file per dialect, never all the dialects in one file.
A case goes in `dialect/<dialect>/` when that dialect is what makes it
interesting, and only a permission outcome that every dialect expresses the
same way belongs in `dialect/engine/inspect_permissions_test.go`, the seam
where they meet.

Reuse `core.GetInspectTestMetadata()` for the catalog rather than building a
fixture: every inspect suite already resolves against it, so a local catalog
is one more thing to keep in step. `core/testutil` holds the statement-tree
walker for assertions that have to reach into subqueries.

Do not put shared cases in `core.GetInspectTestCases` unless the dialects
produce identical trees. They often do not, and `compareResult` is exact, so a
shared table quietly grows per-dialect expectations and stops being shared.

## House rules that bite here

The root `CLAUDE.md` and `CONTRIBUTING.md` carry the repository's rules, and
they win. Two of them have a specific shape in this package:

- Compose what exists before adding a layer. `core.OrUnknown` and
  `core.UnknownStatement` already exist for "a dispatcher fell through", and
  `engine.Inspect` already floors an unrecognised statement. Reach for those
  before inventing a guard.
- Never hand-edit the generated parser files under `*/parser/`. A grammar fix
  is a regeneration, not an edit.

## Cleanup

Probes are scratch. Delete them before staging anything, and stage explicit
paths rather than `git add -A`; a stray probe swept into a commit is easy to do
and embarrassing to undo. Verify with `git status --short` before committing.

## Checks before pushing

```sh
cd dialect
golangci-lint run ./...          # CONTRIBUTING.md: CI also fails on dead code
SELECT_REQUIRE_ANALYZER=1 go test ./...
gofmt -l <the files you touched>
```

`SELECT_REQUIRE_ANALYZER=1` makes the Python analyzer tests fail loudly instead
of skipping, which is the difference between "the suite is green" and "the
suite ran".
