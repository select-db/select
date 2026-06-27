# Git

A SELECT workspace is a folder on disk. Your SQL files, database configs, themes, and lint rules all live as regular files that you can browse, edit, and back up however you like. Git is entirely optional, you can use SELECT without it.

When you work with a team, git becomes useful: connect your workspace to a remote repository, keep files in sync across members and use branches to version your work.

## Getting started

Three ways to set up a workspace:

- **No git**: work locally, no setup needed. You can always add git later.
- **Initialize and publish**: turn the workspace folder into a git repo and push to a remote you provide
- **Link an existing repo**: connect the workspace to a repo that already exists. If the remote has content, SELECT offers to checkout the remote branch or keep local files

## Operations

The git panel exposes standard operations:

- **Stage / Unstage**: select which files to include in the next commit, or stage all at once
- **Commit**: create a commit from staged changes
- **Pull**: fetch and merge from origin. Pull with rebase is also available
- **Push**: push your branch to origin. Force push with lease is available when needed
- **Revert**: discard changes to a file or revert all uncommitted changes
- **Branches**: list branches and switch between them

You can also use the **built-in terminal** (`Ctrl+\``) for any git operation you prefer to run manually.

## Workspace files

A workspace folder contains:

- `.sql` files and their `.metadata.json` sidecars
- `db.config.json` files (credentials should use `$VAR` references, not hardcoded values)
- `.env` files (add to `.gitignore` if they contain secrets)
- `.lint` rules

When git is enabled, these workspace files are tracked. Use a `.gitignore` file
to exclude files from version control. See [.gitignore](/special-files/gitignore/)
for details.

### Personal settings are not tracked

Some settings are **personal to you**, not team policy, so they live in a
per-user config directory **outside** the workspace and are **never committed**:

- `.theme` — your UI appearance ([Theme file](/special-files/theme/))
- `.config` — your keybindings and editor snippets
  ([Config file](/special-files/config/))

This keeps one teammate's color scheme and keybindings from overwriting
everyone else's on pull, and lets your personal settings follow you across every
workspace.

### Workspace settings are synced through the backend

Some settings are team policy but are **not** stored as files. They live on the
workspace and sync through the SELECT backend (like roles and permissions):

- **Execution limits** (`statement_timeout_ms`, `max_result_size_mb`) — edited in
  **Settings → Workspace**.

## Sync

When a workspace is connected to a remote, changes to users, roles, permissions,
and workspace settings (including execution limits) are synced through the SELECT
backend. These are not stored in git but kept in sync across team members through
the server.

> Git syncs your **files**. The SELECT backend syncs your **team settings**
> (users, roles, permissions, execution limits).
