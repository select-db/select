# .gitignore

The `.gitignore` file tells git which files to exclude from version control. Place it at the root of your workspace or in any subfolder.

## Format

One pattern per line. Lines starting with `#` are comments.

```gitignore .gitignore
# Ignore environment files with secrets
.env

# Ignore OS files
.DS_Store
Thumbs.db

# Ignore a specific folder
scratch/
```

## Common patterns for SELECT workspaces

| Pattern       | What it excludes                                   |
|---------------|----------------------------------------------------|
| `.env`        | Environment files containing credentials           |
| `.env.*`      | All environment file variants (`.env.local`, etc.) |
| `*.tmp`       | Temporary files                                    |
| `scratch/`    | A folder for throwaway queries                     |

## What to keep in git

- **`db.config.json`**: safe to commit when credentials use `$VAR` references
- **`.theme`**, **`.config`**, **`.lint`**: shared team settings, should be committed
- **`.sql` files**: your queries, the core of the workspace

## What to exclude

- **`.env`** files with real credentials or secrets
- Temporary or scratch files you don't want to share
- Large data exports

> [!TIP]
> Each subfolder can have its own `.gitignore`. Patterns apply to the folder they live in and all children.

## SELECT reads it too

A workspace is a folder you opened, so it can be a repository with a
`node_modules`, a `target` or a `dist` in it. SELECT reads your `.gitignore` and
skips those **folders** when it builds the file tree and when it watches for
changes, which keeps a large repository fast and keeps the app from running out
of the operating system's file-watch budget.

Only folders are skipped, never files. A gitignored file is still shown and
still read: `.env` is in the shipped `.gitignore` precisely because it holds
secrets, and it is also the file SELECT reads your `$VARIABLES` from.

The patterns understood are the ones people write: comments, `!` negations,
trailing `/`, anchoring with a leading or embedded `/`, `*`, `?`, character
classes and `**`. `.git/info/exclude` and `core.excludesFile` are not read.

`.gitignore` is also the way to keep a folder out of the tree without deleting
it. Adding `scratch/` hides that folder from SELECT as well as from git.
