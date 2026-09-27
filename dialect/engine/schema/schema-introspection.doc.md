# Schema Introspection

When you connect to a database, SELECT fetches its full schema. This metadata is the foundation for several features:

- **Autocompletion**: table, column, function, and type names are suggested as you type SQL
- **Linting**: references to non-existent tables or columns are flagged in real-time
- **Permission checks**: SELECT verifies your role has access to the tables and columns in your query before execution
- **Schema explorer**: browse the full schema tree in the sidebar

## What gets introspected

- **Tables**: columns (name, type, nullable, default), primary keys, foreign keys
- **Views** and **materialized views**: full DDL
- **Indexes**: columns, position, collation, sort direction
- **Triggers**: DDL per table
- **Functions**: name, arguments, return type
- **Types**: enums, composite types, custom types
- **Statistics**: table and index size stats

## How the schema is loaded

SELECT reads the whole schema in a single query on a single connection. The database returns every schema, table, column, key, index, trigger, type, function and setting in one round trip, so a load takes about the same time however many schemas or tables it holds. This matters most on a remote or proxied server, where each round trip costs the network latency.

A few extras run as their own small queries:

- **SQLite**: one more query reads the statistics, only when an `ANALYZE` has left them.
- **MySQL**: one query reads the server variables and one reads the built-in function help. Either is skipped when your account cannot read it.

The DDL shown for a MySQL table or view is rebuilt from `information_schema`. It is equivalent to the output of `SHOW CREATE TABLE`, though not always byte for byte identical.

## How the DDL dump works

SELECT uses two strategies to retrieve schema information, depending on what is available:

### Native dump tool (preferred)

When the dialect's native CLI tool is available on the machine (`pg_dump` for PostgreSQL, `mysqldump` for MySQL), SELECT uses it to produce a full DDL dump. This is the most accurate representation of your schema, including all dialect-specific options and extensions.

### Catalog queries (fallback)

If the native tool is not available, SELECT queries the database's information schema and system catalogs directly, then reconstructs DDL from the metadata. This covers all object types but may miss some dialect-specific features that only the native tool captures.

> [!NOTE]
> When using the fallback strategy, a comment is added to the dump output indicating it was reconstructed from catalog queries.

## Caching

Schema metadata is **cached for 20 minutes** to avoid repeated queries. The cache is invalidated automatically when:

- You modify `datasource.config.json` (change DSN, SSH settings, etc.)
- You reconnect to the database

You can force a refresh from the schema panel. Cached schema dumps are **compressed with zstd** to reduce memory usage.

## Schema explorer

The sidebar shows the full schema tree for each connected database: schemas, tables, columns, types. Clicking a table reveals its columns, keys, and indexes. This tree is built from the cached metadata and updates when the cache refreshes.

![The sidebar tree opened from a database down to one table: its five columns with their types, a key on the primary key and a link on the foreign key, and the index below them.](/shots/schema.light.webp)

To read a table's rows, **double-click** it, or right-click it and choose **View data**. Either opens a temporary SQL tab already attached to the owning database, prefilled with a `SELECT *` limited to the first 100 rows and executed as the tab opens. The statement is a normal SQL file from there on: edit the `LIMIT`, add a `WHERE`, and re-run with `Cmd+Enter`.
