# The dialect method

Shared by the `dialect-finder` and `dialect-fixer` skills. This file is how a
case is drawn, stated and judged; each skill says what to do with the result.
`probes.md` next to it carries the probe templates.

## The dialect package

This package decides what a query is allowed to do, what is wrong with it, and
what to suggest next. It has two parsers: ANTLR inspectors in Go (`Inspect()`,
feeding permissions) and sqlglot in a Python subprocess (lint, completion,
references).

It is covered in layers, **in this order**:

1. **Permissions** -- which rights a statement requires.
2. **Completion** -- what is suggested at a position.
3. **Linter** -- which diagnostics a statement raises, and where.
4. **Resolution** -- hover text, references, go-to-definition.

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
or you measure a hole the seam already closes. `agentprobe` does.

Hold this as a prior, not a conclusion. Confirm it by measurement.

## Enumerating the statement space

"All possible cases" is only checkable if the enumeration has a shape. Every
cell is too many, and most defects need only two values together, so the aim
is every pair of values across two axes tried at least once. A pair with no
case is a gap, and a gap is the thing this work exists to remove.

### The grid

The axes and their values are in `.claude/dialect/grid.json`, per layer. For
permissions, `use` is what the statement does with the table: returns it,
writes it, tests it without returning it (the see boundary), or names it
without reading it. `outer` is the construct around the read, `inner` the one
inside it and `depth` how many there are in all, so pairs compose nesting.
`statements` is how many statements run and whether a later one reads what an
earlier one made, and `read_statement` which of them holds the read.
`requires` rules out the pairs no dialect can express. Policy is not an axis: `agentprobe`
measures every case under deny-all, the data actions without manage,
per-table grants and manage alone.

### Settled rules of the permission model

Some decisions are already taken, so a case that contradicts one is wrong
rather than a finding. Each is stated where it is enforced or pinned, and that
is the copy to read and to change:

- What a tested column needs, and what scopes a write: the doc comment on
  `core.checkTables`.
- What a view is a right on, and that see does not reach through one: the view
  section of `core/testutil/perm_data.go` and the view case in
  `core/testutil/see_data.go`. A view is opaque on purpose, since creating one
  takes manage; revisit only if users ask for it.
- That manage and the row rights do not stand in for each other: the cases
  named for it in the same table.

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

`dialect/cmd/agentprobe` is the first reach: one statement or a JSONL batch,
every layer it covers, and the permission verdict. `dialect/cmd/seesweep`
generates statements and runs them through the engine against a twin database.
Where neither reaches, write a throwaway Go or Python probe; `probes.md`
carries the templates.

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
from a user who did nothing wrong.

Keep a standing list of statements that must keep working: `SELECT 1` (resolves
no table legitimately, on every dialect), a plain `UPDATE`, a CTE, a FROM
subquery, a join with an alias. Reporting a new read is the shape that breaks
these: a name the new code path does not know is virtual resolves to no schema,
and the checker then denies it for every role.

### 6. Ask the real server when the question is about the server

When the question is "what does the database do", start one and ask. Do not
infer from catalog names or documentation.

Executing every operator the completion offers against a real PostgreSQL found
30 it refuses, `point = point` and `json = json` among them. An in-memory
SQLite settles whether a statement parses at all, which decides whether a
misclassification is even reachable.

## Severity

Group by **service** (permissions, linting, completion, hovering, references)
and by **cause** (the ANTLR inspectors, the sqlglot analyzer, metadata
resolution, or the consumer in `core/permissions.go`), then order by severity:

- a **bypass** (ran with no grant) outranks
- a **wrong right** (ran under the wrong permission) outranks
- an **unchecked read** (the write was checked, the read inside was not) outranks
- a **false denial** (fail-closed, annoying, not dangerous) outranks
- a **quality** defect (a wrong suggestion, a misplaced or missing diagnostic,
  a wrong hover).

Split work by **kind of resolution**, not by symptom. Three bugs that all come
from "a nested statement vanished" are one change. A fourth from "a write is
classified as a read" is a different change even in the same file.
