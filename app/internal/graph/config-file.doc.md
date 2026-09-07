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

Keybindings are grouped by the part of the app they belong to: `workbench`,
`editor`, `modal`, and `menu`.

Each binding has:

- **key**: the keystroke, modifiers and a key joined by `+` (e.g.
  `secondary+enter`, `ctrl+shift+-`)
- **command**: what it runs
- **when** (optional): when it applies (e.g. `editorFocus`, `!menuFocus`)

```json .config
{
  "keybindings": {
    "workbench": [
      { "key": "secondary+p", "command": "workbench.openSearch", "when": "!menuFocus" },
      { "key": "secondary+n", "command": "workbench.newSqlFile", "when": "!menuFocus" }
    ],
    "editor": [
      { "key": "secondary+enter", "command": "editor.runQuery", "when": "editorFocus" },
      { "key": "secondary+s", "command": "editor.formatDocument", "when": "editorFocus" }
    ]
  }
}
```

### Modifiers

| Modifier | What it is |
|---|---|
| `secondary` | The shortcut key of the platform: **Cmd** on macOS, **Ctrl** on Linux and Windows. Use this for anything that should work everywhere. |
| `ctrl` | The Control key, on every platform. |
| `alt` | Alt, called Option on macOS. |
| `shift` | Shift. |
| `cmd` | The Command key on macOS; the Super / Windows key elsewhere. |

`control`, `option`, `command`, `super`, `win` and `meta` are accepted as other
spellings of the same modifiers.

> [!NOTE]
> `cmd` used to mean "the shortcut key of the platform" — what `secondary` means
> now. A personal `.config` written before this still works: on Linux and
> Windows a `cmd+…` binding is read as `secondary+…`, and SELECT says so once at
> startup. Rewriting those bindings to `secondary` makes the notice go away.

### Keys

A key is named by **where it is on the keyboard**, not by what shifting it
prints:

- Letters and digits: `a`, `7`
- Punctuation, as printed unshifted on a US layout: `-` `=` `[` `]` `\` `;` `'`
  `,` `.` `/` `` ` ``
- Named keys: `enter`, `escape`, `space`, `tab`, `backspace`, `delete`,
  `insert`, `home`, `end`, `pageup`, `pagedown`, `up`, `down`, `left`, `right`
- Function keys: `f1` … `f24`

So the chord on the minus key with shift held is `shift+-`, never `_`. Naming
the shifted character is refused, with a message saying what to write instead:
the two are the same key on a US layout and different keys elsewhere, and a
binding should not stop working because somebody types on AZERTY.

### When

`when` is a condition over what the app is doing:

`inputFocus`, `editorFocus`, `menuFocus`, `modalOpen`, `leftPanelVisible`,
`rightPanelVisible`, `activeView`, and `os` — which is `macos`, `linux` or
`windows`.

It takes `&&`, `||`, `!`, `==`, `!=` and parentheses:

```json .config
{ "key": "ctrl+-", "command": "workbench.previousTabInGroup", "when": "!menuFocus && os == 'macos'" }
```

`os` is how the shipped keymap gives a command the chord that is conventional on
each platform: walking back through tabs is `ctrl+-` on macOS, where every other
editor puts it, and `secondary+alt+-` on Linux and Windows, where `ctrl+-`
already means zoom out.

### Which binding wins

Your bindings are read after the built-in ones, and **the last binding that
fits wins**. Writing a binding for a chord that already has one replaces it —
there is nothing to remove first.

To take a chord away without putting anything in its place, bind it to nothing.
The keystroke then goes to whatever else wants it — the editor, the browser, the
window manager:

```json .config
{ "key": "secondary+w", "command": "" }
```

### When a binding cannot be read

A binding whose key does not parse is left out, and SELECT reports it at startup
rather than leaving a key that quietly does nothing. The rest of the file is
loaded as usual.

**Available commands:**

- **workbench**: `openSearch`, `closeActiveTab`, `previousTabInGroup`, `nextTabInGroup`, `toggleLeftPanel`, `toggleRightPanel`, `toggleFiles`, `toggleGit`, `toggleSearch`, `zoomIn`, `zoomOut`, `zoomReset`, `openTerminal`, `newSqlFile`
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
