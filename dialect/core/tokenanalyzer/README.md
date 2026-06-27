# tokenanalyzer

SQL analysis (lint, completion context, reference resolution) for selectDb,
powered by [sqlglot](https://github.com/tobymao/sqlglot).

The heavy parsing lives in Python (sqlglot); Go talks to it over a long-running
subprocess. This package is the Go side; the Python side is in [`python/`](./python).

## Architecture

```
Go (this package)                         Python subprocess
─────────────────                         ─────────────────
Analyzer.Call(req) ──── JSON line ──────▶ server.serve()  (main.py)
                  ◀──── JSON line ─────── sqlglot analysis
```

- `Analyzer` (`analyzer.go`) launches and manages one subprocess, one request
  per line of stdin, one response per line of stdout. Safe for concurrent use.
- `LintRunner` (`runner.go`) wraps an `Analyzer` for lint, plus custom rules and
  inline `-- lint-disable` handling.
- Completion/reference helpers live in `dialect/core/references` and take a
  `core.Analyzer`. **If that analyzer is nil, they return `"analyzer is nil"`**,
  see Troubleshooting.

## How the analyzer is discovered

The app wires the analyzer in `app/internal/sqllang` via `resolveAnalyzer()`:

| `APP_ENV` | Looks for | Used by |
|-----------|-----------|---------|
| `dev`     | `dialect/core/tokenanalyzer/python/.venv/bin/python3` + `main.py` (searched from cwd and the binary dir up to the repo root) | local `wails dev` |
| anything else | a sibling `analyzer` binary next to the app executable (PyInstaller one-file build) | packaged app |

`APP_ENV` defaults to `dev` (build-time var in `app/main.go`, also set in
`app/.env`). So **for local development you must create the venv**. Otherwise
there is no analyzer and SQL completion/lint fail with `analyzer is nil`.

## Local setup (dev)

Prerequisites: [uv](https://docs.astral.sh/uv/) and Python ≥ 3.11.

```bash
cd dialect/core/tokenanalyzer/python
uv sync          # creates ./.venv and installs sqlglot
```

That is all the Go side needs. It auto-discovers `./.venv/bin/python3`.
Restart the app (`wails dev`) and completion/lint will work.

Verify the subprocess runs standalone:

```bash
cd dialect/core/tokenanalyzer/python
echo '{"action":"lint","sql":"SELECT 1","dialect":"postgres","schema":{}}' | uv run python main.py
# -> {"diagnostics": [...]}
```

## Tests

```bash
# Python unit tests
cd dialect/core/tokenanalyzer/python
uv run --with pytest pytest analysis/ completion/ lint_rules/ -v

# Go tests (skip automatically when the venv is absent. See testutil.NewTestAnalyzer)
cd dialect && go test ./core/tokenanalyzer/...
```

## Production build (packaged app)

CI builds a standalone binary with PyInstaller and drops it next to the app
executable as `analyzer` (`analyzer.exe` on Windows):

```bash
cd dialect/core/tokenanalyzer/python
uv run --with pyinstaller pyinstaller --onefile --collect-all sqlglot --name analyzer main.py
# -> dist/analyzer  (copied next to the select binary at package time)
```

## Troubleshooting

**`completion failed: analyzer is nil`** (or the same in lint/references):
the Go side found no analyzer. Checklist:

1. `APP_ENV=dev`? (default; check `app/.env`.)
2. Does `dialect/core/tokenanalyzer/python/.venv/bin/python3` exist? If not, run
   `uv sync` in `python/` (see Local setup).
3. Running the packaged app instead of `wails dev`? Then it expects the
   PyInstaller `analyzer` binary beside the executable (see Production build).
