#!/usr/bin/env bash
# Stop hook for fixer runs: publishes the agent's work, so the agent itself
# never talks to GitHub. It runs outside the sandbox, where the action's token
# is, pushes claude/fix-$FIXER_ISSUE and opens the draft pull request from
# .fixer-work/pr.md. Asks the agent back once when something is missing.
set -uo pipefail

[ -n "${FIXER_ISSUE:-}" ] || exit 0
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
input=$(cat)
again=$(jq -r '.stop_hook_active // false' <<<"$input" 2>/dev/null)
[ -s .fixer-work/blocked.md ] && exit 0

ask() {
	[ "$again" = true ] && { echo "publish.sh: $1" >&2; exit 0; }
	jq -n --arg why "$1" '{decision: "block", reason: ("Not published yet: " + $why)}'
	exit 0
}

branch="claude/fix-$FIXER_ISSUE"
[ "$(git rev-parse --abbrev-ref HEAD 2>/dev/null)" = "$branch" ] || exit 0
[ -z "$(git status --porcelain --untracked-files=no)" ] ||
	ask "commit your changes to $branch, then end the run."
[ -n "$(git rev-list origin/dev..HEAD 2>/dev/null)" ] || exit 0

outside=$(git diff --name-only origin/dev...HEAD | awk '
	!/^dialect\// || /^dialect\/[^\/]+\/parser\// || /^dialect\/go\.(mod|sum)$/ ||
	/^dialect\/core\/tokenanalyzer\/python\/(uv\.lock|pyproject\.toml)$/')
[ -z "$outside" ] ||
	ask "the branch changes files the fixer may not touch: $(paste -sd' ' <<<"$outside"). Undo those changes, commit, then end the run."

pr=$(gh pr list --head "$branch" --state open --json number --jq '.[0].number // empty')
if [ -z "$pr" ]; then
	if [ -n "$(.claude/hooks/pr-review-gate.sh required-since origin/dev)" ] &&
		! grep -q '^## Review' .fixer-work/pr.md 2>/dev/null; then
		ask "run the simplify and code-review skills (step 5 of mode \"fix\"), apply what is pertinent, commit, and write .fixer-work/pr.md with its ## Review section, then end the run."
	fi
	[ -s .fixer-work/pr.md ] ||
		ask "write .fixer-work/pr.md (step 6 of mode \"fix\"), then end the run."
fi

for delay in 2 4 8 0; do
	git push -q origin "HEAD:refs/heads/$branch" && break
	[ "$delay" = 0 ] && ask "git push to $branch failed; see the log. End the run."
	sleep "$delay"
done

if [ -z "$pr" ]; then
	gh pr create --draft --base dev --head "$branch" \
		--title "$(head -n1 .fixer-work/pr.md)" --body "$(tail -n +2 .fixer-work/pr.md)" >&2 ||
		ask "the pull request could not be opened; see the log. End the run."
fi
exit 0
