#!/usr/bin/env bash
# Marks the fixer's pull request for an issue ready and asks $FIXER_NOTIFY to
# review it, once CI is green on its head and no fixer run holds the issue.
#   ready.sh <issue number>
set -euo pipefail

issue=$1
branch="claude/fix-$issue"

pr=$(gh pr list --head "$branch" --state open --json number,isDraft,headRefOid \
	--jq '.[0] | select(. != null) | "\(.number) \(.isDraft) \(.headRefOid)"')
[ -n "$pr" ] || { echo "no open pull request on $branch"; exit 0; }
read -r number draft head <<<"$pr"

if gh issue view "$issue" --json labels --jq '.labels[].name' | grep -qx 'fix:in-progress'; then
	echo "a fixer run still holds #$issue"
	exit 0
fi

conclusion=$(gh run list --workflow CI --branch "$branch" --commit "$head" --limit 1 \
	--json status,conclusion --jq '.[0] | select(.status == "completed") | .conclusion')
if [ "$conclusion" != success ]; then
	echo "CI on $head: ${conclusion:-not finished}"
	exit 0
fi

if [ "$draft" = true ]; then
	gh pr ready "$number"
	gh pr edit "$number" --add-reviewer "$FIXER_NOTIFY"
	echo "#$number is ready and $FIXER_NOTIFY is asked to review"
fi
