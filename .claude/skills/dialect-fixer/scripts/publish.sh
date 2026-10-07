#!/usr/bin/env bash
# Stop hook: pushes claude/fix-$FIXER_ISSUE and opens the pull request from .fixer-work/pr.md,
# outside the sandbox. Asks the agent back once when something is missing.
# After a blocked note it still pushes what is committed, so finished work is
# not lost with the runner, but opens no pull request and asks nothing.
# FIXER_MODE=repair replaces the open pull request's branch, leased on
# $FIXER_HEAD, and after a blocked note pushes nothing.
set -uo pipefail

[ -n "${FIXER_ISSUE:-}" ] || exit 0
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
input=$(cat)
again=$(jq -r '.stop_hook_active // false' <<<"$input" 2>/dev/null)
blocked=false
[ -s .fixer-work/blocked.md ] && blocked=true
repair=false
[ "${FIXER_MODE:-fix}" = repair ] && repair=true

ask() {
	{ $blocked || [ "$again" = true ]; } && { echo "publish.sh: $1" >&2; exit 0; }
	jq -n --arg why "$1" '{decision: "block", reason: ("Not published yet: " + $why)}'
	exit 0
}

# A skill counts once it launched its agents; the gate holds that rule.
gate=$AGENT_TOOLS/pr-review-gate.sh

branch="claude/fix-$FIXER_ISSUE"
if [ "$(git rev-parse --abbrev-ref HEAD 2>/dev/null)" != "$branch" ]; then
	$repair && ask "finish the rebase, so that $branch is checked out, then end the run."
	exit 0
fi
[ -z "$(git status --porcelain --untracked-files=no -- dialect)" ] || $blocked ||
	ask "commit your changes to $branch, then end the run."

# A fresh dev, since the agent can move its own origin/dev.
git fetch -q origin dev || ask "git fetch of dev failed; see the log. End the run."
base=$(git rev-parse FETCH_HEAD)
[ -n "$(git rev-list "$base..HEAD")" ] || exit 0

if $repair && $blocked; then
	echo "publish.sh: blocked, so $branch stays as it was" >&2
	exit 0
fi

# CI regenerates the parsers with parsergen.sh and diffs, so the script is the
# one generation file the fixer may not change.
outside=$(git diff --name-only "$base...HEAD" | awk '
	!/^dialect\// || /^dialect\/parsergen\.sh$/ || /^dialect\/go\.(mod|sum)$/ ||
	/^dialect\/core\/tokenanalyzer\/python\/(uv\.lock|pyproject\.toml)$/')
[ -z "$outside" ] ||
	ask "the branch changes files the fixer may not touch: $(paste -sd' ' <<<"$outside"). Undo those changes, commit, then end the run."

pr=$(gh pr list --head "$branch" --state open --json number --jq '.[0].number // empty')
if [ -z "$pr" ] && ! $blocked; then
	# Everything at once: the agent is asked back only once.
	todo=()
	transcript=$(jq -r '.transcript_path // empty' <<<"$input")
	missing=$(jq -r -f "$AGENT_TOOLS/tool-calls.jq" "$transcript" 2>/dev/null | "$gate" missing-since "$base")
	[ -z "$missing" ] ||
		todo+=("run the review skills that have not launched their agents yet (${missing# }), and apply what is pertinent (step 4)")
	[ -s .fixer-work/pr.md ] || todo+=("write .fixer-work/pr.md (step 5)")
	[ -z "$("$gate" required-since "$base")" ] ||
		awk '/^## Review/ { on = 1; next } /^## / { on = 0 } on && NF { found = 1 } END { exit !found }' .fixer-work/pr.md 2>/dev/null ||
		todo+=("fill the ## Review section of .fixer-work/pr.md with the findings applied and those rejected, with the reason (step 5)")
	[ ${#todo[@]} -eq 0 ] || ask "$(printf '%s; ' "${todo[@]}")then commit and end the run."
fi

# With no open pull request, a rerun replaces what a closed one left on the
# branch, leased on the commit seen here; with one open, only a fast-forward,
# unless a repair rebased it.
lease=()
if $repair; then
	[ "$(git rev-parse HEAD)" != "$FIXER_HEAD" ] || exit 0
	lease=(--force-with-lease="refs/heads/$branch:$FIXER_HEAD")
elif [ -z "$pr" ]; then
	lease=(--force-with-lease="refs/heads/$branch:$(git ls-remote origin "refs/heads/$branch" | cut -f1)")
fi
for delay in 2 4 8 0; do
	err=$(git push -q "${lease[@]}" origin "HEAD:refs/heads/$branch" 2>&1) && break
	[ "$delay" = 0 ] && ask "git push to $branch failed: $(grep -E '^( ! |error|fatal)' <<<"$err" | head -n 2 | paste -sd' ') End the run."
	sleep "$delay"
done

if $blocked; then
	at="\`$branch\`"
	[ -z "${GITHUB_REPOSITORY:-}" ] ||
		at="[$branch](${GITHUB_SERVER_URL:-https://github.com}/$GITHUB_REPOSITORY/compare/dev...$branch)"
	printf '\nThe commits so far are on %s.\n' "$at" >>.fixer-work/blocked.md
elif [ -z "$pr" ]; then
	body=$(mktemp)
	tail -n +2 .fixer-work/pr.md | sed '/./,$!d' >"$body"
	gh pr create --base dev --head "$branch" --label agent:fixer \
		--title "$(head -n1 .fixer-work/pr.md)" --body-file "$body" >&2 ||
		ask "the pull request could not be opened; see the log. End the run."
fi
exit 0
