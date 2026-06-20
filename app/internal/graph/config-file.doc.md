# Config File

`.config` controls [query execution](/sql/query-execution/) limits, keybindings,
and editor snippets. It is **split across two locations** by ownership:

| Setting | Owner | Location | Tracked by git |
|---------|-------|----------|----------------|
| `statement_timeout_ms`, `max_result_size_mb` | team | workspace root `.config` | yes |
| `keybindings`, `editor_snippets` | you | per-user config dir `.config` | no |

Execution limits are a project safety policy the team wants enforced, so they
stay in the workspace `.config` and are committed. Keybindings and snippets are
personal, so they live in a per-user `.config` outside every workspace — never
committed, and shared across all your workspaces.

The two files are merged at load time: the workspace wins for execution limits,
and you win for keybindings/snippets.

## Where the per-user file lives

The personal `.config` lives in SELECT's per-user config directory, alongside
your `.theme`:

- **macOS**: `~/Library/Application Support/selectDb/<env>/user-config/.config`
- **Linux**: `$XDG_CONFIG_HOME/selectDb/<env>/user-config/.config` (defaults to
  `~/.config/...`)
- **Windows**: `%APPDATA%\selectDb\<env>\user-config\.config`

You don't need to find it on disk: open it from the global resource menu like any
other file.

## Format

The workspace `.config` holds only execution limits:

```json
{
  "statement_timeout_ms": 30000,
  "max_result_size_mb": 100
}
```

The per-user `.config` holds keybindings and snippets:

```json
{
  "keybindings": { ... },
  "editor_snippets": [ ... ]
}
```

## Execution limits

| Field                    | Default | Description                                         |
|--------------------------|---------|-----------------------------------------------------|
| **statement_timeout_ms** | 30000   | Max query execution time in milliseconds            |
| **max_result_size_mb**   | 100     | Max result set size in MB before truncation         |

## Keybindings

Keybindings are grouped by **context**: `workbench`, `editor`, `modal`, and `menu`.

Each binding has:

- **key**: the shortcut (e.g. `cmd+enter`, `ctrl+\``)
- **command**: the action to trigger
- **when** (optional): condition for the binding to be active (e.g. `editorFocus`, `!menuFocus`)

```json
{
  "keybindings": {
    "workbench": [
      { "key": "cmd+p", "command": "workbench.openSearch", "when": "!menuFocus" },
      { "key": "cmd+n", "command": "workbench.newSqlFile", "when": "!menuFocus" }
    ],
    "editor": [
      { "key": "cmd+enter", "command": "editor.runQuery", "when": "editorFocus" },
      { "key": "cmd+s", "command": "editor.formatDocument", "when": "editorFocus" }
    ]
  }
}
```

**Available contexts and common commands:**

- **workbench**: `openSearch`, `closeActiveTab`, `toggleLeftPanel`, `toggleRightPanel`, `toggleFiles`, `toggleGit`, `toggleSearch`, `zoomIn`, `zoomOut`, `zoomReset`, `openTerminal`, `newSqlFile`
- **editor**: `runQuery`, `formatDocument`, `find`, `replace`, `undo`, `redo`, `toggleLineComment`, `fold`, `unfold`, `goToSymbol`, `quickFix`
- **modal**: `close`
- **menu**: `close`, `selectNext`, `selectPrevious`, `confirm`

## Editor snippets

Define custom SQL snippets that appear in autocompletion:

```json
{
  "editor_snippets": [
    {
      "prefix": "select",
      "body": "SELECT ${2} FROM ${1}",
      "description": "SELECT ... FROM ..."
    }
  ]
}
```

- **prefix**: the trigger text
- **body**: the inserted text, with `${1}`, `${2}` as tab stops
- **description**: shown in the completion menu

## Applying changes

After editing the `.config` file, click **Apply** to reload the configuration. To restore the built-in defaults, click **Reset**.
