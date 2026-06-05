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
| `docs/` | Documentation website source |

## Prerequisites

Go 1.25+ · Node 20+ · [Wails v2](https://wails.io) (desktop) · [uv](https://docs.astral.sh/uv/) (analyzer)

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

**Go modules** (`app` | `backend` | `dialect`): golangci-lint + tests; CI also fails on dead code:
```bash
cd <module>
golangci-lint run ./...
go test ./...
```
> The `app` module embeds the frontend build output; stub it when testing Go only:
> `mkdir -p app/frontend/build && touch app/frontend/build/.keep`

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
