# Getting Started

SELECT is an SQL client shaped like an IDE. Schema-aware completion, real-time linting, [git-based](/workspace/git/) team workspaces with granular DB [permissions](/workspace/permissions/), local and [proxified connections](/databases/proxified-connections/).


This guide takes you from download to your first query in about five minutes.

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

3. Click **Test connection** to validate, then **Save**.

> **Tip:** DSN fields support environment variables (`$VAR_NAME`), resolved from your workspace `.env` file at connection time. Keep credentials in `.env` so you can safely commit your connection config to git.

For full details, see [Connecting a Database](/databases/connecting/).

## 4. Run your first query

1. Right-click your database folder and choose **New file...** (or press **Cmd+N**) to create a `.sql` file.
2. Press **Cmd+Shift+D** to open the database picker and associate the file with the connection you just created.
3. Start typing, you'll get dialect-aware completion for schemas, tables, and columns, with linting as you go:

   ```sql
   SELECT 1;
   ```

4. Press **Cmd+Enter** to run. Results appear in a table below the editor.

You're up and running. From here, swap in a real query against your own tables.

## Where to go next

- [Connecting a Database](/databases/connecting/) — SSH tunnels, proxified connections, and schema introspection.
- [Git Workspaces](/workspace/git/) — version-control your queries and share them with your team.
- [Permissions](/workspace/permissions/) — granular, per-database access control.
- [Query Execution](/sql/query-execution/) — timeouts, result limits, streaming, and caching.
- [SQL Files](/sql/sql-files/) — file associations, snippets, and editor shortcuts.
