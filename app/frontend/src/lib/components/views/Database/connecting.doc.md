# Connecting a Database

SELECT supports **PostgreSQL**, **MySQL**, and **SQLite**.

## Adding a connection

Right-click a folder in the filesystem panel and select **New Database...**. This opens the connection form where you pick your dialect and configure the **DSN**.

DSN format depends on dialect:

| Dialect    | DSN format                                                              |
|------------|-------------------------------------------------------------------------|
| PostgreSQL | `host=localhost port=5432 user=admin password=*** dbname=mydb sslmode=require` |
| MySQL      | `user:password@tcp(host:3306)/dbname?parseTime=true`                    |
| SQLite     | `file:./local.db` or `/path/to/database.sqlite`                        |

> DSN fields support **environment variables** (`$VAR_NAME`). SELECT resolves them from your workspace `.env` file at connection time, keeping credentials out of config files.

## Testing the connection

Hit **Test connection** to validate your setup before saving. SELECT attempts to connect and reports any errors inline.

## Configuration storage

Each database lives in its own folder inside the workspace, named `db-<uuid>`. The folder contains a `db.config.json` with the connection settings:

```
workspace/
  db-e731d451-.../
    db.config.json
    queries/
      report.sql
  db-a4f29c10-.../
    db.config.json
```

You can organize SQL files and subfolders inside each database folder. The `db.config.json` file is managed by SELECT and updated when you change connection settings in the form.

> Keep credentials in `.env` and reference them with `$VAR_NAME` in the DSN so you can safely commit `db.config.json` to git.

For team databases where credentials should not live on individual machines, see [Proxified Connections](/databases/proxified-connections/).
