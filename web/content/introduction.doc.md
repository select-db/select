# Getting Started

SELECT is an SQL client shaped like an IDE. Schema-aware completion, real-time linting, [git-based](/docs/workspace/git/) team workspaces with granular DB [permissions](/docs/workspace/permissions/), local and [proxified connections](/docs/databases/proxified-connections/).


This guide takes you from download to your first query in about five minutes.

![The application: the schema tree opened down to a table's columns, a query mid-completion with the column list over it, its results in the grid below, and the chat answering a question about that same query.](/shots/hero.wide.light.png)

## 1. Download

Current release: **v0.0.1**

See [all releases](https://github.com/select-db/select/releases).

Each platform ships as a `.zip`. The links below always resolve to the **latest** release; for a specific version swap `latest/download` for `download/vX.Y.Z`.

| Platform | Download |
| --- | --- |
| macOS (Apple Silicon) | [selectDb-darwin-arm64.zip](https://github.com/select-db/select/releases/latest/download/selectDb-darwin-arm64.zip) |
| macOS (Intel) | [selectDb-darwin-amd64.zip](https://github.com/select-db/select/releases/latest/download/selectDb-darwin-amd64.zip) |
| Windows (x64) | [selectDb-windows-amd64.zip](https://github.com/select-db/select/releases/latest/download/selectDb-windows-amd64.zip) |
| Linux (x64) | [selectDb-linux-amd64.zip](https://github.com/select-db/select/releases/latest/download/selectDb-linux-amd64.zip) |

## 2. Install

### macOS

Unzip and drag **SELECT** to your Applications folder.

### Windows

Unzip and run `SELECT.exe`.

### Linux

Unzip and run the `select` binary.

## 3. Connect your first database

SELECT supports **PostgreSQL**, **MySQL**, and **SQLite**.

1. Right-click a folder in the filesystem panel and choose **New Database...**.
2. Pick your dialect and fill in the **DSN**:

   | Dialect    | DSN format |
   |------------|------------|
   | PostgreSQL | `host=localhost port=5432 user=admin password=*** dbname=mydb sslmode=require` |
   | MySQL      | `user:password@tcp(host:3306)/dbname?parseTime=true` |
   | SQLite     | `file:./local.db` or `/path/to/database.sqlite` |

3. Click **Test connection** to check it. There is nothing to save: the form writes `db.config.json` for you once the connection is valid.

![The connection form: dialect, name, and a DSN written as $WAREHOUSE_DSN rather than a literal connection string.](/shots/getting-started.database.light.png)

> [!TIP]
> DSN fields support environment variables (`$VAR_NAME`), resolved from your workspace `.env` file at connection time. Keep credentials in `.env` so you can safely commit your connection config to git.

For full details, see [Connecting a Database](/docs/databases/connecting/).

## 4. Run your first query

1. Right-click your database folder and choose **New file...** to create a `.sql` file. (In a hurry? `Cmd+N` opens a [scratch tab](/docs/sql/sql-files/#scratch-files) you can save later.)
2. Press `Cmd+Shift+D` to open the database picker and associate the file with the connection you just created.

   ![The database picker open over a SQL file, with the warehouse connection ticked.](/shots/getting-started.picker.light.png)
3. Start typing. Completion comes from the schema SELECT just read off your
   database, so after `FROM` you get your own tables, and after a table you get
   its columns with their types. Misspell one and the linter marks it before
   you run anything:

   ```sql
   SELECT * FROM your_table LIMIT 10;
   ```

4. Press `Cmd+Enter` to run. Results appear in a table below the editor.

   ![A query run: twelve rows of weekly revenue in the result grid, with the time it took beside the row count.](/shots/getting-started.result.light.png)

You're up and running.

## Where to go next {.cards}

- [Connecting a Database](/docs/databases/connecting/): SSH tunnels, proxified connections, and schema introspection.
- [Git Workspaces](/docs/workspace/git/): version-control your queries and share them with your team.
- [Permissions](/docs/workspace/permissions/): granular, per-database access control.
- [Query Execution](/docs/sql/query-execution/): timeouts, result limits, streaming, and caching.
- [SQL Files](/docs/sql/sql-files/): file associations, snippets, and editor shortcuts.
