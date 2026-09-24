#!/usr/bin/env bash
# Labels the fixer's pull request fix:reviewed when a review run's transcript
# shows every skill the review gate requires for its size, and removes the
# label otherwise, so ready.sh never marks unreviewed code ready.
#   reviewed.sh <pull request> <execution file>
set -euo pipefail

pr=$1
transcript=${2:-}

lines=$(gh pr view "$pr" --json additions,deletions --jq '.additions + .deletions')
required=$(.claude/hooks/pr-review-gate.sh required "$lines")
if [ -z "$required" ]; then
	gh pr edit "$pr" --add-label fix:reviewed
	exit 0
fi

if [ ! -s "$transcript" ]; then
	gh pr edit "$pr" --remove-label fix:reviewed >/dev/null 2>&1 || true
	gh pr comment "$pr" --body "@$FIXER_NOTIFY the review run left no transcript, so its reviews cannot be checked and this stays a draft: $FIXER_RUN_URL. Add fix:reviewed to let it go ready once CI is green."
	exit 0
fi

# Loading a skill only reads its instructions. Both skills fan out to
# sub-agents, so a skill counts once an Agent or Task call follows it.
ran=$(jq -r '.. | objects | select(.type? == "tool_use")
	| if .name == "Skill" then "skill " + ((.input.skill // "") | sub(".*:"; "")) else .name end' "$transcript" |
	awk '$1 == "skill" { current = $2; next } ($1 == "Agent" || $1 == "Task") && current != "" { print current }' | sort -u)
echo "Skills that ran their agents: $(paste -sd' ' <<<"${ran:-none}")" >>"${GITHUB_STEP_SUMMARY:-/dev/stderr}"

missing=''
for want in $required; do
	grep -qx "$want" <<<"$ran" || missing="$missing $want"
done

if [ -z "$missing" ]; then
	gh pr edit "$pr" --add-label fix:reviewed
else
	gh pr edit "$pr" --remove-label fix:reviewed >/dev/null 2>&1 || true
	gh pr comment "$pr" --body "@$FIXER_NOTIFY these reviews never launched their agents:${missing}. This stays a draft: $FIXER_RUN_URL. Add fix:reviewed to let it go ready once CI is green."
fi
