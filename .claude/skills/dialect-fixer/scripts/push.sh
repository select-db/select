#!/usr/bin/env bash
# Pushes the current commit to claude/fix-$FIXER_ISSUE and nowhere else. The
# fixer's only way to write to the remote, so the branch it can reach is fixed
# here rather than by a command prefix the agent could extend with a refspec.
set -euo pipefail

: "${FIXER_ISSUE:?FIXER_ISSUE is not set}"
case "$FIXER_ISSUE" in *[!0-9]*) echo "FIXER_ISSUE is not a number: $FIXER_ISSUE" >&2; exit 2 ;; esac

branch="claude/fix-$FIXER_ISSUE"
if [ "$(git rev-parse --abbrev-ref HEAD)" != "$branch" ]; then
	echo "on $(git rev-parse --abbrev-ref HEAD), not $branch" >&2
	exit 2
fi
for delay in 2 4 8 16; do
	git push origin "HEAD:refs/heads/$branch" && exit 0
	sleep "$delay"
done
echo "push to $branch failed" >&2
exit 1
