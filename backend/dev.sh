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
#   ./dev.sh start                 generate, then run the backend server
#
# Prerequisites: a reachable, migrated Postgres (POSTGRES_DSN, loaded from .env
# by the Go binaries). `generate`/`start` introspect the live schema, so run
# `./dev.sh migrate up` first after any schema change.

set -euo pipefail

# Resolve this script's absolute path before changing directory (BASH_SOURCE is
# relative to the caller's cwd), then operate on the backend module.
SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
cd "$(dirname "$SCRIPT_PATH")"

BOLD=$(tput bold 2>/dev/null || true)
NORMAL=$(tput sgr0 2>/dev/null || true)
BLUE='\033[0;34m'
GREEN='\033[0;32m'
NC='\033[0m'

step() { echo -e "\n${BLUE}${BOLD}==>${NC}${BOLD} $1${NORMAL}"; }
done_() { echo -e "${GREEN}${BOLD}✔${NC}${BOLD} $1${NORMAL}"; }

usage() {
  # Print the header comment block (everything from line 3 up to the first
  # non-comment line), stripping the leading "# ".
  awk 'NR<=2 {next} /^#/ {sub(/^# ?/, ""); print; next} {exit}' "$SCRIPT_PATH"
  exit "${1:-0}"
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
  start) start ;;
  -h | --help | help | "") usage 0 ;;
  *)
    echo "unknown command: '$1'" >&2
    usage 1 >&2
    ;;
esac
