#!/usr/bin/env bash
# Commits the ledger and pushes it to the memory branch. The finder's only way
# to write to the remote, so the branch it can reach is fixed here.
set -euo pipefail

memory=${FINDER_MEMORY:-.finder-memory}
message=${1:-checkpoint}

# The worktree's .git is a pointer file; git runs here with the token, so it
# must lead to this repository and not to a config someone else wrote.
[ "$(git -C "$memory" rev-parse --path-format=absolute --git-common-dir)" = "$(git rev-parse --path-format=absolute --git-common-dir)" ] ||
	{ echo "$memory is not a worktree of this repository" >&2; exit 2; }

git -C "$memory" add -A
if ! git -C "$memory" diff --cached --quiet; then
	git -C "$memory" commit -q -m "finder: $message"
fi
[ -n "$(git -C "$memory" rev-parse -q --verify HEAD)" ] || exit 0

# Pushed even when nothing new was committed, so a push that failed at the
# last checkpoint is retried at this one.
if [ "${FINDER_DRY_RUN:-0}" = 1 ]; then
	echo "dry run: committed, not pushed"
	exit 0
fi
for delay in 2 4 8 16; do
	git -C "$memory" push -q origin HEAD:refs/heads/agent/finder-memory && exit 0
	sleep "$delay"
done
echo "push to agent/finder-memory failed" >&2
exit 1
