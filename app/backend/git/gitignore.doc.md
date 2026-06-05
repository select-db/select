# .gitignore

The `.gitignore` file tells git which files to exclude from version control. Place it at the root of your workspace or in any subfolder.

## Format

One pattern per line. Lines starting with `#` are comments.

```
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

> Each subfolder can have its own `.gitignore`. Patterns apply to the folder they live in and all children.
