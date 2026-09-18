#!/usr/bin/env bash
# Gate that keeps a session from finishing after it opened a pull request
# without running the codebase-design and code-review skills over the diff.
# State lives under the git directory, so it is per clone and never committed.
set -uo pipefail

mode=${1:-}
input=$(cat)

# A pull request opened outside this session, from the Claude Code UI or the
# GitHub web interface, produces no tool call, so nothing arms the gate.
if ! command -v jq >/dev/null 2>&1; then
	[ "$mode" = stop ] && printf '{"systemMessage":"pr-review-gate: jq is missing, so the review gate did not run."}\n'
	exit 0
fi

json() { printf '%s' "$input" | jq -r "$1" 2>/dev/null; }

root=${CLAUDE_PROJECT_DIR:-$PWD}
gitdir=$(git -C "$root" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
session=$(json '.session_id // empty')
state="$gitdir/pr-review-gate/${session:-nosession}"

required='codebase-design code-review'
# Blocking forever would wedge the session, so give up after this many tries.
max_blocks=3

case "$mode" in
pr-opened)
	# The Bash matcher's `if` filter does not reliably narrow to the pull
	# request command, so check the command here. An MCP tool call carries no
	# command at all.
	cmd=$(json '.tool_input.command // empty')
	if [ -n "$cmd" ] && ! printf '%s' "$cmd" | grep -Eq '(^|[^[:alnum:]_-])gh[[:space:]]+pr[[:space:]]+create'; then
		exit 0
	fi
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
	missing=''
	for want in $required; do
		[ -f "$state/$want" ] || missing="$missing $want"
	done
	if [ -z "$missing" ]; then
		rm -rf "$state"
		exit 0
	fi
	blocks=$(cat "$state/blocks" 2>/dev/null || echo 0)
	if [ "$blocks" -ge "$max_blocks" ]; then
		jq -n --arg missing "${missing# }" --arg tries "$max_blocks" '{
			systemMessage: ("pr-review-gate: giving up after " + $tries + " attempts; these skills never ran: " + $missing)
		}'
		rm -rf "$state"
		exit 0
	fi
	echo $((blocks + 1)) >"$state/blocks"
	jq -n --arg missing "${missing# }" '{
		decision: "block",
		reason: ("A pull request was opened in this session and these skills have not run over the diff yet: " + $missing + ". Run each one against the branch base, report the findings, then finish.")
	}'
	;;
esac
exit 0
