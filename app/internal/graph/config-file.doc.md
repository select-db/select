# Config File

`.config` is a **personal** file: it holds your keybindings and editor snippets.
It is **not** part of any workspace and is never committed to git. It lives in
SELECT's per-user config directory and follows you across every workspace.

> [!IMPORTANT]
> Execution limits (`statement_timeout_ms`, `max_result_size_mb`) are **not** in
> `.config`. They are workspace-level team policy and are configured in
> **Settings → Workspace** (see [Execution limits](#execution-limits) below).

## Where the file lives

- **macOS**: `~/Library/Application Support/selectDb/<env>/user-config/.config`
- **Linux**: `$XDG_CONFIG_HOME/selectDb/<env>/user-config/.config` (defaults to
  `~/.config/...`)
- **Windows**: `%APPDATA%\selectDb\<env>\user-config\.config`

You don't need to find it on disk: open it from **Settings → Config**.

## Format

```json .config
{
  "keybindings": { ... },
  "editor_snippets": [ ... ]
}
```

## Execution limits

Execution limits are **workspace** settings, shared with everyone in the
workspace (synced through the SELECT backend, like roles and permissions), and
edited in **Settings → Workspace**:

| Field                    | Default | Description                                         |
|--------------------------|---------|-----------------------------------------------------|
| **statement_timeout_ms** | 30000   | Max query execution time in milliseconds            |
| **max_result_size_mb**   | 100     | Max result set size in MB before truncation (max 250)|

## Keybindings

Keybindings are grouped by **context**: `workbench`, `editor`, `modal`, and `menu`.

Each binding has:

- **key**: the shortcut (e.g. `cmd+enter`, `ctrl+\``)
- **command**: the action to trigger
- **when** (optional): condition for the binding to be active (e.g. `editorFocus`, `!menuFocus`)

```json .config
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

```json .config
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

After editing `.config` (in **Settings → Config**), click **Apply** to reload
the configuration. To restore the built-in defaults, click **Reset**.
