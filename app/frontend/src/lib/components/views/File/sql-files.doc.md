# SQL Files

SQL files are the main workspace artifact in SELECT. They are persisted on disk as regular `.sql` files and can be version-controlled with [git](/docs/workspace/git/).

## Creating files

Right-click a folder in the filesystem panel and select **New file...**. Files are created with an auto-incremented name (`#1.sql`, `#2.sql`, ...) and can be renamed freely.

## Scratch files

`Cmd+N` opens a scratch tab instead: `[temp].sql`, which lives in the tab and not on disk. It inherits the database of whatever you were looking at, so you can ask a question the moment you have it, without naming a file first.

Most queries start there and end there, which is the point. Save one to the workspace when it turns out to be worth keeping, and it becomes an ordinary `.sql` file with everything that follows from that: a name, git history, lint rules, a review.

## Database association

Each SQL file can be associated with one or more database connections. Press `Cmd+Shift+D` to open the database picker and select your targets. These associations are stored in a `.metadata.json` sidecar file next to the SQL file. When executing, SELECT runs the query against the selected database.

## Editing

The editor provides **dialect-aware autocompletion** for keywords, schemas, tables, columns, and functions. Linting runs in real-time using both built-in checks and any custom rules from your `.lint` file.

Available editor shortcuts are defined in the `.config` file under `keybindings.editor`. Custom **snippets** can be added to speed up common patterns.
