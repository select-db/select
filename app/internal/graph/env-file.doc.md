# Environment File

Each folder in your workspace can contain a `.env` file. SELECT loads these files and makes their variables available for [variable substitution](/docs/sql/variables/) in database DSN strings and SQL files.

## Format

Standard `KEY=VALUE` pairs, one per line:

```ini .env
PG_HOST=localhost
PG_PORT=5432
PG_USER=admin
PG_PASS=secret
PG_DB=myapp
```

**Supported syntax:**

- **Comments** with `#` at the start of a line
- **Quoted values** with single or double quotes: `KEY="value with spaces"`
- **Multi-line values** using quotes that span multiple lines
- **Escaped characters** in quoted values: `\n`, `\t`, `\\`
- **Inline comments** after unquoted values: `KEY=value # this is ignored`
- Keys must match `[A-Za-z_][A-Za-z0-9_]*`

## Usage in DSN

Reference variables with `$VAR_NAME` in your database connection string. Multiple variables can be used in a single field:

```
host=$PG_HOST port=$PG_PORT user=$PG_USER password=$PG_PASS dbname=$PG_DB
```

SELECT resolves all `$VAR` references at connection time. This keeps credentials out of `db.config.json`.

## Usage in SQL

Variables work in SQL files too. Reference a `$VAR_NAME` and SELECT substitutes it before execution. You can also reference another SQL file in the same folder by name: `$my_query` resolves to the content of `my_query.sql`.

## Folder inheritance

Variables are resolved by walking up the folder hierarchy. A `.env` file in a parent folder is available to all children. If the same key exists in both parent and child, the **parent takes precedence**.

## Applying changes

After editing the `.env` file, click **Apply** to reload variables. To restore the built-in defaults, click **Reset**.

> [!WARNING]
> Variables from `.env` are **not available in proxified mode**. Proxified connections store credentials server-side.
