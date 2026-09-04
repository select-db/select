#!/usr/bin/env bash
#
# dev.sh — developer toolkit for the whole repo. One entrypoint so the commands
# are somewhere findable instead of in three README sections.
#
#   ./dev.sh app start             run the desktop app with live reload
#   ./dev.sh app build             build the desktop app into app/build/bin
#   ./dev.sh app package           package it for this platform
#   ./dev.sh app bindings          regenerate the frontend bindings from Go
#   ./dev.sh app test              Go tests, then the frontend's check and lint
#   ./dev.sh app e2e               the Playwright suite against a server build
#   ./dev.sh app migrate up        apply pending migrations to the current server
#   ./dev.sh app migrate down      roll back the last one
#   ./dev.sh app migrate reset     roll back every one
#   ./dev.sh app migrate status    show migration state
#   ./dev.sh app migrate new <name>  scaffold a migration
#
#   ./dev.sh web start             build the site, serve on :3333, rebuild on save
#   ./dev.sh web build             build the site into web/dist once
#   ./dev.sh web shots             recapture the product screenshots
#
#   ./dev.sh backend start         db up, migrate, generate, run the server
#   ./dev.sh backend test          go test ./... (wants the dev DB up)
#   ./dev.sh backend db up         start the dev DB container (docker compose)
#   ./dev.sh backend db down       stop it (add -v to wipe the data volume)
#   ./dev.sh backend migrate up    apply all pending migrations (goose)
#   ./dev.sh backend migrate down  roll back the last migration
#   ./dev.sh backend migrate reset roll back every migration
#   ./dev.sh backend migrate status  show migration state
#   ./dev.sh backend migrate new <name>  scaffold a new migration
#   ./dev.sh backend generate      codegen: apigen (schema -> sql+glue) then sqlc
#
#   ./dev.sh dialect test          go test ./... for the dialect module
#   ./dev.sh test                  every module's tests, the way CI runs them
#
# `app test` and `app e2e` compile the app's Go code, which links the webview.
# On Linux that wants the GTK3 headers: apt install libgtk-3-dev
# libwebkit2gtk-4.1-dev. macOS and Windows need nothing extra.
#
# The dev database runs in Docker (see backend/docker-compose.yml and
# Dockerfile.postgres: Postgres 17 with pg_partman baked in), published on host
# port 5431 to match the default POSTGRES_DSN in backend/.env. Requires Docker;
# the Go binaries read .env themselves.

set -euo pipefail

# Resolve this script's absolute path before changing directory (BASH_SOURCE is
# relative to the caller's cwd), so every command below can name its own
# directory and this works from anywhere in the tree.
SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
ROOT="$(dirname "$SCRIPT_PATH")"

BOLD=$(tput bold 2>/dev/null || true)
NORMAL=$(tput sgr0 2>/dev/null || true)
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

step() { echo -e "\n${BLUE}${BOLD}==>${NC}${BOLD} $1${NORMAL}"; }
done_() { echo -e "${GREEN}${BOLD}✔${NC}${BOLD} $1${NORMAL}"; }
warn() { echo -e "${YELLOW}${BOLD}!${NC}${BOLD} $1${NORMAL}" >&2; }

usage() {
  # Print the header comment block (everything from line 3 up to the first
  # non-comment line), stripping the leading "# ".
  awk 'NR<=2 {next} /^#/ {sub(/^# ?/, ""); print; next} {exit}' "$SCRIPT_PATH"
  exit "${1:-0}"
}

# --- website --------------------------------------------------------------
# One generator builds the whole site, docs and marketing pages alike. It needs
# the repo, not just web/: every .doc.md lives beside the code it documents and
# every screenshot beside the capture that took it.

WEB_PORT="${WEB_PORT:-3333}"

# `go run .`, never `go run main.go`: the generator is several files, and naming
# one of them fails on the symbols in the others.
web_build() {
  step "Web — building the site"
  (cd "$ROOT/web/generate" && go run .)
  done_ "web build (web/dist)"
}

web_start() {
  # A generator left running from an earlier session serves a stale build out of
  # the same directory this one writes to, and the two fight over it. Cheaper to
  # say so than to debug a site that rebuilds itself back to yesterday.
  if pgrep -f "generate -serve" >/dev/null 2>&1; then
    warn "another 'dev.sh web start' is already running:"
    pgrep -af "generate -serve" >&2 || true
    warn "stop it first:  pkill -f 'generate -serve'"
    exit 1
  fi
  step "Web — serving on http://localhost:$WEB_PORT (watching for changes)"
  (cd "$ROOT/web/generate" && go run . -serve -port "$WEB_PORT")
}

# The shots live in the app's Taskfile because taking them means building and
# driving the app. Through the task, never `npm run shots`: playwright launches
# build/bin/select-server without ever building it, so a stale binary does not
# fail the run, it republishes pictures of the previous build.
web_shots() {
  command -v wails3 >/dev/null 2>&1 || {
    echo "wails3 not found. Install it: go install github.com/wailsapp/wails/v3/cmd/wails3@latest" >&2
    exit 1
  }
  step "Web — recapturing screenshots (builds the app first)"
  (cd "$ROOT/app" && wails3 task shots)
  done_ "web shots — look at the diff before committing the images"
}

web() {
  local sub="${1:-}"; shift || true
  case "$sub" in
    build) web_build ;;
    start) web_start ;;
    shots) web_shots ;;
    *) echo "unknown web subcommand: '${sub:-}' (want: build|start|shots)" >&2; exit 1 ;;
  esac
}

# --- app ------------------------------------------------------------------
# Everything here goes through wails3, which owns the build: it generates the
# bindings, builds the frontend into the binary and knows the platform tags.

require_wails3() {
  command -v wails3 >/dev/null 2>&1 || {
    echo "wails3 not found. Install it: go install github.com/wailsapp/wails/v3/cmd/wails3@latest" >&2
    exit 1
  }
}

# The build tags the app's own Go code needs. Linux pins GTK3 + WebKit2GTK 4.1
# because wails v3 defaults to GTK4, which the distros we ship to do not carry;
# every other platform needs none. Same rule as PLATFORM_TAGS in app/Taskfile.yml.
app_tags() { [[ "$(uname -s)" == Linux ]] && echo "gtk3"; }

app_task() {
  require_wails3
  step "App — $1"
  (cd "$ROOT/app" && wails3 task "$1")
  done_ "app $1"
}

# The same three checks CI runs over the app, in the order that fails fastest.
app_test() {
  local tags; tags="$(app_tags)"
  step "App — Go tests${tags:+ (tags: $tags)}"
  if [[ -n "$tags" ]]; then
    (cd "$ROOT/app" && go test -tags "$tags" ./internal/...)
  else
    (cd "$ROOT/app" && go test ./internal/...)
  fi

  step "App — frontend typecheck (svelte-check)"
  (cd "$ROOT/app/frontend" && npm run check)

  step "App — frontend lint (eslint)"
  (cd "$ROOT/app/frontend" && npm run lint)

  done_ "app test"
}

# Every server the app knows has its own SQLite database. up/down/reset/status
# take an optional domain and otherwise target the current server, which is the
# one the app would open if you launched it now. `new` writes a file and touches
# no database at all.
app_migrate() {
  local sub="${1:-}"; shift || true
  case "$sub" in
    up|down|reset|status|new)
      step "App — migrate $sub"
      (cd "$ROOT/app" && CGO_ENABLED=0 go run -tags server ./internal/cmd/migrate "$sub" "$@")
      done_ "app migrate $sub"
      ;;
    *) echo "unknown migrate subcommand: '${sub:-}' (want: up|down|reset|status|new)" >&2; exit 1 ;;
  esac
}

app() {
  local sub="${1:-}"; shift || true
  case "$sub" in
    start)    app_task dev ;;
    build)    app_task build ;;
    package)  app_task package ;;
    bindings) app_task generate:bindings ;;
    test)     app_test ;;
    e2e)      app_task test:e2e ;;
    migrate)  app_migrate "$@" ;;
    *) echo "unknown app subcommand: '${sub:-}' (want: start|build|package|bindings|test|e2e|migrate)" >&2; exit 1 ;;
  esac
}

# --- backend --------------------------------------------------------------
# db up/down only boot or shut down the container. Migrations and codegen are
# orchestrated by `start`; the app logs pg_partman status itself on boot.
COMPOSE_FILE="$ROOT/backend/docker-compose.yml"
DB_SERVICE="db"

compose() { docker compose -f "$COMPOSE_FILE" "$@"; }

require_docker() {
  command -v docker >/dev/null 2>&1 || { echo "docker not found. Install Docker to run the dev database." >&2; exit 1; }
  docker info >/dev/null 2>&1 || { echo "Docker daemon not running. Start Docker and retry." >&2; exit 1; }
}

# db_up starts the container and waits for the healthcheck.
db_up() {
  require_docker
  step "DB — starting container"
  compose up -d "$DB_SERVICE"
  echo "  waiting for healthy…"
  local cid status=""
  for _ in $(seq 1 60); do
    cid="$(compose ps -q "$DB_SERVICE")"
    [[ -n "$cid" ]] && status="$(docker inspect -f '{{.State.Health.Status}}' "$cid" 2>/dev/null || true)"
    [[ "$status" == healthy ]] && break
    sleep 1
  done
  [[ "$status" == healthy ]] || { echo "db container not healthy; see: docker compose -f $COMPOSE_FILE logs $DB_SERVICE" >&2; exit 1; }
  done_ "db up"
}

# db_down stops the container. Extra args pass through (e.g. -v wipes the volume).
db_down() {
  require_docker
  step "DB — stopping container"
  compose down "$@"
  done_ "db down"
}

db() {
  local sub="${1:-}"; shift || true
  case "$sub" in
    up)   db_up ;;
    down) db_down "$@" ;;
    *)    echo "unknown db subcommand: '${sub:-}' (want: up|down)" >&2; exit 1 ;;
  esac
}

migrate() {
  local sub="${1:-}"
  case "$sub" in
    up | down | reset | status)
      step "migrate:$sub"
      (cd "$ROOT/backend" && go run ./cmd/server "migrate:$sub")
      ;;
    new)
      local name="${2:-}"
      [[ -n "$name" ]] || { echo "migrate new: missing <name>" >&2; exit 1; }
      step "migrate:new $name"
      (cd "$ROOT/backend" && go run ./cmd/server migrate:new "$name")
      ;;
    *)
      echo "unknown migrate subcommand: '${sub:-}' (want: up|down|reset|status|new)" >&2
      exit 1
      ;;
  esac
  done_ "migrate $sub"
}

# generate runs the two-phase codegen in order: apigen introspects the live
# schema and writes the .sql queries + Go glue, then sqlc compiles those .sql
# files into db/generated (which the glue references).
generate() {
  step "Codegen — apigen (schema → sql + glue)"
  (cd "$ROOT/backend" && go run ./cmd/apigen generate)

  step "Codegen — sqlc (sql → db/generated)"
  (cd "$ROOT/backend" && ./scripts/generate_queries.sh)

  done_ "generate"
}

backend_start() {
  db_up
  migrate up
  generate
  step "Starting backend server"
  (cd "$ROOT/backend" && go run ./cmd/server)
}

# The suite talks to a real Postgres -- partitioning and pg_partman are most of
# what it is checking, and neither survives being faked -- so it wants the dev
# container up. `./dev.sh backend db up` first.
backend_test() {
  step "Backend — go test ./..."
  (cd "$ROOT/backend" && go test ./...)
  done_ "backend test"
}

backend() {
  local sub="${1:-}"; shift || true
  case "$sub" in
    start)    backend_start ;;
    test)     backend_test ;;
    generate) generate ;;
    db)       db "$@" ;;
    migrate)  migrate "$@" ;;
    *) echo "unknown backend subcommand: '${sub:-}' (want: start|test|db|migrate|generate)" >&2; exit 1 ;;
  esac
}

# --- dialect --------------------------------------------------------------

dialect_test() {
  step "Dialect — go test ./..."
  (cd "$ROOT/dialect" && go test ./...)
  done_ "dialect test"
}

dialect() {
  local sub="${1:-}"; shift || true
  case "$sub" in
    test) dialect_test ;;
    *) echo "unknown dialect subcommand: '${sub:-}' (want: test)" >&2; exit 1 ;;
  esac
}

# --- everything -----------------------------------------------------------
# What CI runs, minus the end-to-end suite: that one builds a binary and drives
# a browser, and belongs to a decision to wait for it (`./dev.sh app e2e`).
test_all() {
  app_test
  dialect_test
  backend_test
  done_ "all tests"
}

case "${1:-}" in
  app)
    shift
    app "$@"
    ;;
  web)
    shift
    web "$@"
    ;;
  dialect)
    shift
    dialect "$@"
    ;;
  test) test_all ;;
  backend)
    shift
    backend "$@"
    ;;
  -h | --help | help | "") usage 0 ;;
  *)
    echo "unknown command: '$1'" >&2
    usage 1 >&2
    ;;
esac
