# Lint Rules

SELECT lints your SQL as you type. Not with a list of regexes. The statement is
parsed into a syntax tree and checked against the schema SELECT introspected
from your database, so the linter knows that `orders.custmer_id` is not a
column, that `id` is ambiguous across two joined tables, and that the CTE you
renamed at the top is now unused.

Findings appear inline in the editor and in the problems list. Nothing runs
against the database to produce them.

> [!NOTE]
> Rules that resolve names and types need the schema. If SELECT has not
> introspected the database yet, or the connection is closed, those rules stay
> quiet rather than guessing. You will still get the ones that read the
> statement alone.

## Severities

| Severity | Means |
|----------|-------|
| **error** | The statement is wrong. It will fail, or it will run and not do what it says. |
| **warning** | Almost certainly a mistake, but there is a reading where you meant it. |
| **hint** | Style and habit. Correct SQL that the next reader will thank you for changing. |

Every severity is adjustable per workspace, including down to `off`, in the
[`.lint` file](/docs/sql/lint-rules/lint-file/).

## Names and references

These need the schema.

| Rule | Severity | Fires on |
|------|----------|----------|
| `unknown-table` | warning | A table or view that is not in the schema. |
| `unknown-column` | warning | A column that is not on any table in scope. |
| `ambiguous-column` | warning | An unqualified column that exists on more than one table in the query. |
| `unused-cte` | hint | A CTE that is defined and never referenced. |
| `unknown-function` | warning | A function the dialect does not define. |

## Types

These need the schema too.

| Rule | Severity | Fires on |
|------|----------|----------|
| `type-mismatch-comparison` | warning | Comparing two expressions of incompatible types. |
| `type-mismatch-arithmetic` | error | Arithmetic on a non-numeric column. |
| `type-mismatch-argument` | warning | A function argument whose type is not what the function takes. |
| `in-list-type-mismatch` | warning | An `IN` list value that cannot match the column's type. |
| `coalesce-type-mismatch` | warning | `COALESCE` over arguments of inconsistent types. |
| `enum-value-mismatch` | warning | Comparing an enum column to a value outside its allowed set. |
| `missing-argument` | error | A function called with no arguments where it requires one. |

## NULL

The rules that catch the mistake everyone makes at least once.

| Rule | Severity | Fires on |
|------|----------|----------|
| `null-equality` | error | `= NULL`, which is never true. Use `IS NULL`. |
| `null-inequality` | error | `<> NULL`, same problem. |
| `null-in-not-in` | warning | `NOT IN` over a list containing `NULL`, which returns no rows. |
| `null-in-list` | warning | `NULL` inside an `IN` list, where it can never match. |
| `coalesce-single-arg` | warning | `COALESCE` with one argument, which does nothing. |

## Structure and aggregation

| Rule | Severity | Fires on |
|------|----------|----------|
| `agg-in-where` | error | An aggregate in `WHERE`. Use `HAVING`. |
| `agg-in-join` | error | An aggregate in a `JOIN ... ON` condition. |
| `having-without-group-by` | error | `HAVING` with no `GROUP BY`. |
| `window-in-where` | error | A window function in `WHERE`. |
| `duplicate-column` | warning | Two output columns with the same name. |
| `ordinal-out-of-range` | error | `GROUP BY 4` when the select list has three columns. |
| `offset-without-limit` | error | `OFFSET` with no `LIMIT`. |
| `limit-without-order-by` | hint | `LIMIT` with no `ORDER BY`: the rows you get are arbitrary. |
| `subquery-order-by` | hint | `ORDER BY` inside a subquery, where it is discarded. |

## Predicates and style

| Rule | Severity | Fires on |
|------|----------|----------|
| `contradictory-predicate` | error | A condition that can never be true. |
| `tautological-predicate` | warning | A condition that is always true. |
| `division-by-zero` | error | Division by a literal zero. |
| `reversed-between` | error | `BETWEEN 10 AND 1`, which matches nothing. |
| `empty-in-list` | error | `IN ()`. |
| `duplicate-in-value` | warning | The same value twice in an `IN` list. |
| `like-no-wildcard` | warning | `LIKE` with no `%` or `_`, which is just `=`. |
| `like-leading-wildcard` | hint | `LIKE '%foo'`, which cannot use an index. |
| `trailing-comma` | error | A trailing comma in the select list. |
| `unquoted-uppercase` | warning | An unquoted uppercase identifier, which the database will fold to lowercase. |

## Turning a rule off

Set a severity to `off` in the [`.lint` file](/docs/sql/lint-rules/lint-file/)
to change it for the whole workspace, or for a set of paths:

```json .lint
[
  {
    "files": ["migrations/**"],
    "rules": { "unquoted-uppercase": "off" }
  }
]
```

For a single statement, a comment does it:

```sql
-- lint-disable-next-line like-leading-wildcard
SELECT * FROM customers WHERE email LIKE '%@example.com';
```

- `-- lint-disable-next-line <rule-id>` suppresses the rule on the following line.
- `-- lint-disable <rule-id>` anywhere in the file suppresses it for the whole file.

## Your own rules

The built-in rules are about SQL being correct. Rules about what your team
allows are yours to write, as regex patterns in the
[`.lint` file](/docs/sql/lint-rules/lint-file/): no `DROP TABLE` outside
`migrations/`, no `SELECT *` in a committed query. That file is committed with the
workspace, so the rule your team agreed on is the rule everyone gets.
