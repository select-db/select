#!/usr/bin/env bash
# Writes what the agent reads from GitHub into .fixer-work, because the agent
# runs with no GitHub token. Issue comments from people without write access
# are left out: they are the easiest place to plant instructions.
#   context.sh <issue> [failed run id]
set -euo pipefail

issue=$1
run=${2:-}
mkdir -p .fixer-work
grep -qx '.fixer-work/' .git/info/exclude 2>/dev/null || echo '.fixer-work/' >>.git/info/exclude

{
	gh issue view "$issue" --json number,title,body --jq '"# #\(.number) \(.title)\n\n\(.body)"'
	echo
	echo "## Comments from maintainers and the finder"
	gh api "repos/$GITHUB_REPOSITORY/issues/$issue/comments" --paginate --jq '.[]
		| select(.author_association == "OWNER" or .author_association == "MEMBER"
			or .author_association == "COLLABORATOR" or .user.login == "github-actions[bot]")
		| "\n### \(.user.login), \(.created_at)\n\n\(.body)"'
} >.fixer-work/issue.md

if [ -n "$run" ]; then
	gh run view "$run" --log-failed >.fixer-work/ci.log 2>&1 || true
fi
