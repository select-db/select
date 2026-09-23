#!/usr/bin/env bash
# Labels the fixer's pull request fix:reviewed when an agent run's transcript
# shows every skill the review gate requires, and removes the label otherwise,
# so ready.sh never marks unreviewed code ready.
#   reviewed.sh <pull request> <execution file>
set -euo pipefail

pr=$1
transcript=${2:-}

if [ ! -s "$transcript" ]; then
	gh pr edit "$pr" --remove-label fix:reviewed >/dev/null 2>&1 || true
	gh pr comment "$pr" --body "@$FIXER_NOTIFY the fixer run left no transcript, so its reviews cannot be checked and this stays a draft: $FIXER_RUN_URL. Add fix:reviewed to let it go ready once CI is green."
	exit 0
fi

# Counts a call, not its outcome: a review that errored still counts.
ran=$(jq -r '.. | objects | select(.type? == "tool_use" and .name? == "Skill") | .input.skill // empty | sub(".*:"; "")' \
	"$transcript" | sort -u)
echo "Skills run: $(paste -sd' ' <<<"${ran:-none}")" >>"${GITHUB_STEP_SUMMARY:-/dev/stderr}"

missing=''
for want in $(.claude/hooks/pr-review-gate.sh required); do
	grep -qx "$want" <<<"$ran" || missing="$missing $want"
done

if [ -z "$missing" ]; then
	gh pr edit "$pr" --add-label fix:reviewed
else
	gh pr edit "$pr" --remove-label fix:reviewed >/dev/null 2>&1 || true
	gh pr comment "$pr" --body "@$FIXER_NOTIFY the fixer did not run${missing} over its last push, so this stays a draft: $FIXER_RUN_URL. Add fix:reviewed to let it go ready once CI is green."
fi
