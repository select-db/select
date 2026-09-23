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

Assign `$FINDER_NOTIFY` on every `sev:bypass`. Mention `@$FINDER_NOTIFY` in the
first line of every other body.

## Title

`<area>: <what goes wrong> (<dialects>)`, stated as the defect, not the case:

- `permission: reads inside RETURNING are not checked (postgresql, sqlite)`
- `completion: CTE columns missing after a dot in a correlated subquery (mysql)`

## Body

Write full sentences. The reader is a fixer agent or a maintainer who has not
seen this run.

````markdown
@<FINDER_NOTIFY>

**Finding** `f-...` · layer `permission` · severity `unchecked-read` · oracle `judgment`
**Grid position** `read_site=returning` (shared by every case below)

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

---
Filed by the dialect finder, run <RUN_URL>. Close with one `verdict:` label and
a one-line reason: the finder reads both.
````

Keep the table to at most ten rows. Past ten, say how many more cases the
ledger holds for this finding.

## Commands

```sh
gh issue create --title "..." --body-file body.md \
  --label agent:finder,bug,area:permission,dialect:postgresql,sev:unchecked-read,oracle:judgment,needs-triage
gh issue create ... --assignee "$FINDER_NOTIFY"      # sev:bypass only
gh issue comment <n> --body-file more.md            # growing an open finding
```

Write the body to a file under `$RUNNER_TEMP` or `.finder-work/` first; a body
passed inline breaks on the first backtick.
