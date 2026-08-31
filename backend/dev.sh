#!/usr/bin/env bash
#
# dev.sh — backend developer toolkit. One entrypoint for the full local flow so
# no step gets missed. Run from anywhere; it operates on the backend module.
#
#   ./dev.sh migrate up            apply all pending migrations (goose)
#   ./dev.sh migrate down          roll back the last migration
#   ./dev.sh migrate reset         roll back every migration
#   ./dev.sh migrate status        show migration state
#   ./dev.sh migrate new <name>    scaffold a new migration
#
#   ./dev.sh generate              codegen: apigen (schema -> sql+glue) then sqlc
#   ./dev.sh db up                 start the dev DB container (docker compose)
#   ./dev.sh db down               stop the dev DB container (add -v to wipe data)
#   ./dev.sh start                 db up, migrate, generate, run the backend server
#
# The dev database runs in Docker (see docker-compose.yml + Dockerfile.postgres:
# Postgres 17 with pg_partman baked in), published on host port 5431 to match the
# default POSTGRES_DSN in backend/.env. Requires Docker; the Go binaries read
# .env themselves.

set -euo pipefail

# Resolve this script's absolute path before changing directory (BASH_SOURCE is
# relative to the caller's cwd), then operate on the backend module.
SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
cd "$(dirname "$SCRIPT_PATH")"

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

# --- dev database (Docker) ------------------------------------------------
# db up/down only boot or shut down the container. Migrations and codegen are
# orchestrated by `start`; the app logs pg_partman status itself on boot.
COMPOSE_FILE="$(dirname "$SCRIPT_PATH")/docker-compose.yml"
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

# db dispatches the symmetric up/down subcommands.
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
      go run ./cmd/server "migrate:$sub"
      ;;
    new)
      local name="${2:-}"
      [[ -n "$name" ]] || { echo "migrate new: missing <name>" >&2; exit 1; }
      step "migrate:new $name"
      go run ./cmd/server migrate:new "$name"
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
  go run ./cmd/apigen generate

  step "Codegen — sqlc (sql → db/generated)"
  ./scripts/generate_queries.sh

  done_ "generate"
}

start() {
  db_up
  migrate up
  generate
  step "Starting backend server"
  go run ./cmd/server
}

case "${1:-}" in
  migrate)
    shift
    migrate "$@"
    ;;
  generate) generate ;;
  db)
    shift
    db "$@"
    ;;
  start) start ;;
  -h | --help | help | "") usage 0 ;;
  *)
    echo "unknown command: '$1'" >&2
    usage 1 >&2
    ;;
esac
