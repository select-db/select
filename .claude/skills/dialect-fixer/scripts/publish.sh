#!/usr/bin/env bash
# Stop hook: pushes claude/fix-$FIXER_ISSUE and opens the draft from .fixer-work/pr.md,
# outside the sandbox. Asks the agent back once when something is missing.
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

# A skill counts once it launched its agents; the gate holds that rule.
gate=$AGENT_TOOLS/pr-review-gate.sh

branch="claude/fix-$FIXER_ISSUE"
[ "$(git rev-parse --abbrev-ref HEAD 2>/dev/null)" = "$branch" ] || exit 0
[ -z "$(git status --porcelain --untracked-files=no -- dialect)" ] ||
	ask "commit your changes to $branch, then end the run."

# A fresh dev, since the agent can move its own origin/dev.
git fetch -q origin dev || ask "git fetch of dev failed; see the log. End the run."
base=$(git rev-parse FETCH_HEAD)
[ -n "$(git rev-list "$base..HEAD")" ] || exit 0

outside=$(git diff --name-only "$base...HEAD" | awk '
	!/^dialect\// || /^dialect\/[^\/]+\/parser\// || /^dialect\/go\.(mod|sum)$/ ||
	/^dialect\/core\/tokenanalyzer\/python\/(uv\.lock|pyproject\.toml)$/')
[ -z "$outside" ] ||
	ask "the branch changes files the fixer may not touch: $(paste -sd' ' <<<"$outside"). Undo those changes, commit, then end the run."

pr=$(gh pr list --head "$branch" --state open --json number --jq '.[0].number // empty')
if [ -z "$pr" ]; then
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

for delay in 2 4 8 0; do
	git push -q origin "HEAD:refs/heads/$branch" && break
	[ "$delay" = 0 ] && ask "git push to $branch failed; see the log. End the run."
	sleep "$delay"
done

if [ -z "$pr" ]; then
	body=$(mktemp)
	tail -n +2 .fixer-work/pr.md | sed '/./,$!d' >"$body"
	gh pr create --draft --base dev --head "$branch" \
		--title "$(head -n1 .fixer-work/pr.md)" --body-file "$body" >&2 ||
		ask "the pull request could not be opened; see the log. End the run."
fi
exit 0
