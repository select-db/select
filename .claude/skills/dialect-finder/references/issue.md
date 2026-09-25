# Filing an issue

## Labels

`scripts/labels.sh` creates the whole set before every run, so a label named
here always exists. Every finder issue carries:

- `agent:finder` and `bug`
- one `area:` label: `permission`, `completion`, `lint` or `resolution`
- one `dialect:` label per dialect the group reproduces on
- one `sev:` label: `bypass`, `wrong-right`, `unchecked-read`, `false-denial`
  or `quality`
- one `oracle:` label: `cross-dialect` when the dialects disagree on the same
  position, `judgment` when the finding rests on the expectation alone
- `needs-triage` whenever the oracle is `judgment`

## Title

`<area>: <what goes wrong> (<dialects>)`, stated as the defect, not the case:

- `permission: reads inside RETURNING are not checked (postgresql, sqlite)`
- `completion: CTE columns missing after a dot in a correlated subquery (mysql)`

## Body

Write full sentences. The reader is a fixer agent or a maintainer who has not
seen this run.

````markdown
**Layer** `permission` · severity `unchecked-read` · oracle `judgment`
**Shared by every case** `read_site=returning`

### What goes wrong
One paragraph: the user scenario. Who writes this, what they hold, what
happens that should not.

### Reproduce
| dialect | SQL | expected | measured |
| --- | --- | --- | --- |
| postgresql | `DELETE FROM t1 WHERE c1 = 1 RETURNING c2` | delete on main.t1, select on main.t1.c1, select on main.t1.c2 | delete on main.t1 |

Catalog: `core.GetInspectTestMetadata()` unless stated.
`go run ./cmd/agentprobe -json -dialect postgresql -sql '...'` from `dialect/`.

### Why this is a bug
The expectation, argued from SQL semantics and the permission model, with the
settled rule it rests on if one applies (see `.claude/dialect/method.md`).

### The case against
The strongest argument that this is correct, from the adversarial review, and
why it does not hold.

### Related
Earlier findings or precedents on the same axes, if any.
````

Keep the table to at most ten rows. Past ten, say how many more cases there are.

## Filing

The workflow files after the run. Write each issue to
`.finder-work/file/<short-name>.md`: its headers, a blank line, then the body.

```
title: permission: MERGE floors to an unknown statement (postgresql)
labels: agent:finder,bug,area:permission,dialect:postgresql,sev:wrong-right,oracle:judgment,needs-triage

### What goes wrong
...
```

Only the labels above are accepted. The workflow adds the mention, the run
link and the closing instructions, and assigns every `sev:bypass`.
