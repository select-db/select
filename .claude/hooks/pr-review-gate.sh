#!/usr/bin/env bash
# Gate that keeps a session from finishing after it opened a pull request
# without running the codebase-design and code-review skills over the diff.
# State lives under the git directory, so it is per clone and never committed.
set -uo pipefail

mode=${1:-}
input=$(cat)

command -v jq >/dev/null 2>&1 || exit 0

json() { printf '%s' "$input" | jq -r "$1" 2>/dev/null; }

root=${CLAUDE_PROJECT_DIR:-$PWD}
gitdir=$(git -C "$root" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
session=$(json '.session_id // empty')
state="$gitdir/pr-review-gate/${session:-nosession}"

required='codebase-design code-review'

case "$mode" in
pr-opened)
	mkdir -p "$state" || exit 0
	: >"$state/pending"
	jq -n '{
		hookSpecificOutput: {
			hookEventName: "PostToolUse",
			additionalContext: "A pull request was just opened. Before finishing this turn, run the codebase-design skill over the diff of this branch against its base, then run the code-review skill with that base as the fixed point. Report both sets of findings and act on them."
		}
	}'
	;;
skill-ran)
	[ -f "$state/pending" ] || exit 0
	name=$(json '.tool_input.skill // empty')
	name=${name##*:}
	for want in $required; do
		[ "$name" = "$want" ] && : >"$state/$want"
	done
	;;
stop)
	[ -f "$state/pending" ] || exit 0
	[ "$(json '.stop_hook_active // false')" = "true" ] && exit 0
	missing=''
	for want in $required; do
		[ -f "$state/$want" ] || missing="$missing $want"
	done
	if [ -n "$missing" ]; then
		jq -n --arg missing "${missing# }" '{
			decision: "block",
			reason: ("A pull request was opened in this session and these skills have not run over the diff yet: " + $missing + ". Run each one against the branch base, report the findings, then finish.")
		}'
		exit 0
	fi
	rm -f "$state/pending"
	;;
esac
exit 0
