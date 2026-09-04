# Contributing to SELECT

How the repo is laid out and what CI expects before a change merges. By
participating you agree to the [Code of Conduct](./CODE_OF_CONDUCT.md). For
security issues, **don't open a public issue**; see [SECURITY.md](./SECURITY.md).

## Layout

Monorepo of Go modules + a Svelte frontend + a Python analyzer:

| Path | What it is |
|------|------------|
| `app/` | Wails desktop client: Go backend (`app/internal`) + Svelte frontend (`app/frontend`) |
| `backend/` | Hosted proxy and sync server (Go) |
| `dialect/` | Shared SQL engine: per-dialect parsing/introspection + the Python token analyzer (`dialect/core/tokenanalyzer/python`) |
| `toolkit/` | Small shared Go utilities |
| `web/` | select-db.com: the site generator, docs manifest, theme and marketing pages |
| `docs/` | Internal repo docs (proposals, runbooks) — not published |

## Prerequisites

Go 1.25+ · Node 20+ · [Wails v3](https://v3.wails.io) (desktop) · [uv](https://docs.astral.sh/uv/) (analyzer)

## Branches

- **`dev`**: default/integration branch; **all PRs target `dev`**. May be briefly unstable.
- **`main`**: always-stable; maintainers promote `dev` to `main` and tag releases (`vX.Y.Z`) from it.

## Workflow

1. Branch from `dev`; open your PR against `dev` (not a tag).
2. Keep PRs focused; explain what changed and why, and link related issues.
3. CI runs per-module on changed paths. Make the checks below pass for what you touched.
4. A maintainer reviews and merges; releases are cut from `vX.Y.Z` tags.

## Checks (mirror CI)

**Frontend** (`app/frontend`):
```bash
npm ci && npm run check && npm run lint
```
> The frontend calls the Go services through the generated bindings in
> `app/frontend/src/lib/bindings`. They are committed; regenerate and commit them
> whenever a bound service's signature changes:
> `cd app && wails3 task generate:bindings`

**Go modules** (`app` | `backend` | `dialect`): golangci-lint + tests; CI also fails on dead code:
```bash
cd <module>
golangci-lint run ./...
go test ./...
```
> The `app` module embeds the frontend build output; stub it when testing Go only:
> `mkdir -p app/frontend/build && touch app/frontend/build/.keep`
>
> On Linux the desktop app links against GTK3/WebKit2GTK 4.1 through cgo, so
> building, linting or testing the `app` module needs `libgtk-3-dev` and
> `libwebkit2gtk-4.1-dev` installed and the `gtk3` build tag
> (`go test -tags gtk3 ./internal/...`).

**End-to-end** (`app`):
```bash
cd app && wails3 task test:e2e
```
> The suite drives the real app built with wails' `server` tag: the same Go
> services, bindings and events, served over HTTP instead of embedded in a
> webview, so Playwright can drive it with no display server. It cannot cover
> anything that needs a native window — zoom, dialogs, menus — and it runs
> against Chromium rather than WebKit/WebView2, so those stay manual.
>
> Specs live in `app/frontend/tests/e2e`. `npx playwright test --ui` gives the
> watch mode; the binary it drives comes from `wails3 task build:server`.
>
> Each run gets a throwaway data directory, seeded by `internal/cmd/e2eseed`
> with a migrated database, a user, a workspace and its files — without which
> the app only ever shows a login screen. Sign-in itself is not part of it: the
> tokens live in the OS keyring, so specs use the `signIn` fixture, which emits
> the same `login` event the Go side emits once it finds them.

**Python analyzer** (`dialect/core/tokenanalyzer/python`):
```bash
uv sync
uv run --with pytest pytest analysis/ completion/ lint_rules/ -v
```

## Commits

Follow [Conventional Commits](https://www.conventionalcommits.org): `type(scope): summary`.
Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`.
Example: `fix(dialect): handle empty MySQL result set`.

- Imperative summary; explain the *why* in the body when it isn't obvious.
- Never commit secrets, local paths, or generated artifacts.

## Issues

Use the issue templates. Anything security-related goes through [SECURITY.md](./SECURITY.md).
