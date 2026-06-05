# SQL Files

SQL files are the main workspace artifact in SELECT. They are persisted on disk as regular `.sql` files and can be version-controlled with [git](/workspace/git/).

## Creating files

Right-click a folder in the filesystem panel and select **New file...**, or press **Cmd+N**. Files are created with an auto-incremented name (`#1.sql`, `#2.sql`, ...) and can be renamed freely.

## Database association

Each SQL file can be associated with one or more database connections. Press **Cmd+Shift+D** to open the database picker and select your targets. These associations are stored in a `.metadata.json` sidecar file next to the SQL file. When executing, SELECT runs the query against the selected database.

## Editing

The editor provides **dialect-aware autocompletion** for keywords, schemas, tables, columns, and functions. Linting runs in real-time using both built-in checks and any custom rules from your `.lint` file.

Available editor shortcuts are defined in the `.config` file under `keybindings.editor`. Custom **snippets** can be added to speed up common patterns.
