# Variables

SELECT supports `$VARIABLE` references in SQL files. Variables are resolved from [`.env` files](/special-files/env/), other SQL files, or prompted from the user at runtime.

## Environment variables

Define variables in a `.env` file and reference them with `$VAR_NAME`:

```
# .env
API_KEY=sk-123456
TENANT=acme
```

```sql
SELECT * FROM users WHERE tenant = $TENANT;
```

SELECT substitutes variables before execution with proper SQL escaping: text values are single-quoted, numbers are inserted as-is, booleans become `TRUE`/`FALSE`.

## SQL file references

Reference another SQL file in the same folder by name. `$setup` resolves to the content of `setup.sql`, letting you compose queries from reusable fragments:

```sql
-- setup.sql
CREATE TEMP TABLE active_users AS
SELECT id FROM users WHERE active = true;

-- main.sql
$setup
SELECT * FROM active_users WHERE created_at > '2024-01-01';
```

SQL file references are expanded recursively, so referenced files can themselves contain `$VAR` references.

## Runtime variable prompting

When a `$VARIABLE` is not found in any `.env` file or SQL file, SELECT prompts you to fill it in before execution. A form appears with one field per unresolved variable.

Each variable can be typed:

| Type        | SQL output                      |
|-------------|----------------------------------|
| **text**    | `'value'` (single-quoted, escaped) |
| **integer** | `42` (unquoted)                 |
| **decimal** | `3.14` (unquoted)               |
| **boolean** | `TRUE` or `FALSE`               |
| **date**    | `'2024-01-15'`                  |
| **timestamp** | `'2024-01-15 09:30:00'`       |
| **time**    | `'09:30:00'`                    |

Runtime variable values are **persisted per file per tab**, so they pre-fill on subsequent runs. This is useful for queries you run repeatedly with different parameters.

> Runtime variables turn any SQL file into a reusable parameterized query without modifying `.env`.
