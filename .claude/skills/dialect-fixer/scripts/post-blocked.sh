#!/usr/bin/env bash
# Posts the note the agent left in .fixer-work/blocked.md, since the agent has
# no GitHub token. On an issue it also labels it fix:blocked and assigns it.
#   post-blocked.sh issue|pr <number>
set -euo pipefail

kind=$1
number=$2
[ -s .fixer-work/blocked.md ] || exit 0
gh "$kind" comment "$number" --body-file .fixer-work/blocked.md
if [ "$kind" = issue ]; then
	gh issue edit "$number" --add-label fix:blocked --add-assignee "$FIXER_NOTIFY"
fi
