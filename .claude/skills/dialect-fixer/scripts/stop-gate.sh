#!/usr/bin/env bash
# Stop hook for unattended fixer runs. The review skills end with a summary,
# and a headless run ends with whatever the agent says last, so without this
# the run stops on that summary before acting on it. No-op outside the fixer.
set -uo pipefail

[ -n "${FIXER_ISSUE:-}" ] || exit 0
input=$(cat)
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false' 2>/dev/null)" = true ] && exit 0
[ -s .fixer-work/blocked.md ] && exit 0

branch="claude/fix-$FIXER_ISSUE"
[ "$(git rev-parse --abbrev-ref HEAD 2>/dev/null)" = "$branch" ] || exit 0
[ -n "$(git rev-list origin/dev..HEAD 2>/dev/null)" ] || exit 0

todo=''
[ "$(git rev-parse HEAD)" = "$(git rev-parse -q --verify "origin/$branch" 2>/dev/null)" ] ||
	todo="push your commits with /home/runner/work/select/select/.claude/skills/dialect-fixer/scripts/push.sh"
if [ -n "$(.claude/hooks/pr-review-gate.sh required-since origin/dev)" ] && [ ! -s .fixer-work/review.md ]; then
	todo="${todo:+$todo, then }apply the review findings you judge pertinent, rerun the checks, push, and write .fixer-work/review.md (steps 6 and 7 of mode \"fix\")"
fi
[ -n "$todo" ] || exit 0
jq -n --arg todo "$todo" '{decision: "block", reason: ("The fixer run is not finished: " + $todo + ". Then end the run.")}'
