#!/usr/bin/env bash
# Gate that keeps a session from finishing after it opened a pull request
# without running the simplify and code-review skills over the diff. A pull
# request changing fewer than $small lines is not gated.
# State lives under the git directory, so it is per clone and never committed.
set -uo pipefail

mode=${1:-}
required='simplify code-review'
small=50

# The skills a pull request changing $1 lines needs; unknown counts as large.
required_for() { [ -n "${1:-}" ] && [ "$1" -lt "$small" ] || echo "$required"; }

# Reads tool calls, one per line ("Skill <name>" or the tool's name), and
# prints the skills that ran: loading one only reads its instructions, and both
# fan out to sub-agents, so a skill counts once an Agent or Task call follows.
fanned_out() {
	awk '$1 == "Skill" { current = $2; sub(/.*:/, "", current); next }
		($1 == "Agent" || $1 == "Task") && current != "" { print current }' | sort -u
}

# The skills HEAD needs as a pull request against $1.
required_since() {
	required_for "$(git -C "${CLAUDE_PROJECT_DIR:-$PWD}" diff --shortstat "$1...HEAD" 2>/dev/null |
		awk '{ n = 0; for (i = 1; i < NF; i++) if ($(i + 1) ~ /^(insertion|deletion)/) n += $i; print n }')"
}

# publish.sh shares this list, this threshold and this rule through these modes.
[ "$mode" = required-since ] && { required_since "${2:-origin/dev}"; exit 0; }
[ "$mode" = fanned-out ] && { fanned_out; exit 0; }

input=$(cat)

command -v jq >/dev/null 2>&1 || exit 0

json() { printf '%s' "$input" | jq -r "$1" 2>/dev/null; }

root=${CLAUDE_PROJECT_DIR:-$PWD}
gitdir=$(git -C "$root" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
session=$(json '.session_id // empty')
state="$gitdir/pr-review-gate/${session:-nosession}"

case "$mode" in
pr-opened)
	base=$(json '.tool_input.base // empty')
	[ -n "$(required_since "origin/${base:-dev}")" ] || exit 0
	mkdir -p "$state" || exit 0
	: >"$state/pending"
	jq -n '{
		hookSpecificOutput: {
			hookEventName: "PostToolUse",
			additionalContext: "A pull request was just opened. Before finishing this turn, run the simplify skill over the diff of this branch against its base, then run the code-review skill with that base as the fixed point. Report both sets of findings and act on them."
		}
	}'
	;;
skill-ran)
	[ -f "$state/pending" ] || exit 0
	name=$(json '.tool_input.skill // empty')
	echo "Skill $name" >>"$state/calls"
	;;
agent-ran)
	[ -f "$state/pending" ] || exit 0
	echo Agent >>"$state/calls"
	;;
stop)
	[ -f "$state/pending" ] || exit 0
	[ "$(json '.stop_hook_active // false')" = "true" ] && exit 0
	ran=$(fanned_out 2>/dev/null <"$state/calls")
	missing=''
	for want in $required; do
		grep -qx "$want" <<<"$ran" || missing="$missing $want"
	done
	if [ -n "$missing" ]; then
		jq -n --arg missing "${missing# }" '{
			decision: "block",
			reason: ("A pull request was opened in this session and these skills have not run over the diff yet: " + $missing + ". Invoke each one against the branch base and let it launch its agents, report the findings, then finish.")
		}'
		exit 0
	fi
	rm -f "$state/pending" "$state/calls"
	;;
esac
exit 0
