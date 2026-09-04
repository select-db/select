# SELECT

An SQL client shaped like an IDE. Schema-aware completion, real-time linting,
git-based team workspaces with granular per-database permissions, and local or
proxified connections to PostgreSQL, MySQL, and SQLite.

## Development

`./dev.sh` is the entrypoint for everything local, from the repo root:

```
./dev.sh app start                  the desktop app, with live reload
./dev.sh app build                  build it into app/build/bin
./dev.sh app package                package it for this platform
./dev.sh app bindings               regenerate the frontend bindings from Go
./dev.sh app test                   Go tests, then the frontend's check and lint
./dev.sh app e2e                    the Playwright suite against a server build
./dev.sh app migrate up|down|reset|status [domain]   the app's SQLite migrations
./dev.sh app migrate new <name>     scaffold one

./dev.sh backend start              dev database, migrations, codegen, server
./dev.sh backend test               go test ./... (wants the dev database up)
./dev.sh backend generate           sqlc and the syncer's generated code
./dev.sh backend db up|down         the dev database container (docker compose)
./dev.sh backend migrate up         apply pending migrations (goose)
./dev.sh backend migrate down       roll back the last one
./dev.sh backend migrate reset      roll back every one
./dev.sh backend migrate status     show migration state
./dev.sh backend migrate new <name> scaffold a migration

./dev.sh dialect test               the SQL engine's tests

./dev.sh test                       every module's tests, the way CI runs them
```

`./dev.sh` on its own lists the rest.
