#!/usr/bin/env bash
# The finder's only way to write to GitHub. It runs outside Claude Code's
# sandbox, with the token the sandbox withholds, so it posts nothing but a body
# the agent wrote under .finder-work/, links resolved.
#   file.sh issue "<title>" <label,label,...> <body file> [assign]
#   file.sh comment <issue number> <body file>
set -euo pipefail

[ "${FINDER_DRY_RUN:-0}" = 1 ] && { echo "dry run: filing nothing" >&2; exit 2; }

body_file() {
	local path
	path=$(realpath -e -- "$1") || { echo "no body file: $1" >&2; exit 2; }
	case "$path" in
	"$PWD"/.finder-work/*) [ -f "$path" ] && { echo "$path"; return; } ;;
	esac
	echo "the body must be a file under .finder-work/: $1" >&2
	exit 2
}

case "${1:-}" in
issue)
	[ $# -ge 4 ] || { echo "usage: file.sh issue <title> <labels> <body file> [assign]" >&2; exit 2; }
	[[ $3 =~ ^[a-z0-9:_-]+(,[a-z0-9:_-]+)*$ ]] || { echo "bad labels: $3" >&2; exit 2; }
	args=(--title "$2" --label "$3" --body-file "$(body_file "$4")")
	[ "${5:-}" = assign ] && args+=(--assignee "$FINDER_NOTIFY")
	gh issue create "${args[@]}" | grep -oE '[0-9]+$'
	;;
comment)
	[ $# -eq 3 ] && [[ $2 =~ ^[0-9]+$ ]] || { echo "usage: file.sh comment <issue number> <body file>" >&2; exit 2; }
	gh issue comment "$2" --body-file "$(body_file "$3")"
	;;
*)
	echo "usage: file.sh issue|comment ..." >&2
	exit 2
	;;
esac
