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

![The connection form for a PostgreSQL database: dialect, name, the proxy checkbox, the DSN and SSH tunnel modes, and a DSN written from $VAR references.](/shots/dbform.local.light.png)

> [!TIP]
> DSN fields support **environment variables** (`$VAR_NAME`), resolved from your workspace `.env` at connection time. Write the DSN as `host=$PG_HOST password=$PG_PASS ...` and the password never enters `db.config.json`, which is what makes that file safe to commit.

## Testing the connection

Hit **Test connection** to check your setup. SELECT attempts to connect and reports any errors inline. There is no save step: the form writes `db.config.json` once the settings are valid.

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

You can organize SQL files and subfolders inside each database folder. The `db.config.json` file is managed by SELECT and written as you change connection settings in the form. It holds the DSN exactly as you typed it, so a DSN built from `$VAR` references contains no secrets and belongs in git with the rest of the workspace.

For team databases where credentials should not live on individual machines, see [Proxified Connections](/docs/databases/proxified-connections/).
