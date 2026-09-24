#!/usr/bin/env bash
# Labels the fixer's pull request fix:reviewed when the fixer run's transcript
# shows every skill the review gate requires for its size, and removes the
# label otherwise, so ready.sh never marks unreviewed code ready.
#   reviewed.sh <pull request> <execution file>
#   reviewed.sh required <pull request>   prints the skills it needs
set -euo pipefail

gate=.claude/hooks/pr-review-gate.sh
required_for_pr() {
	"$gate" required "$(gh pr view "$1" --json additions,deletions --jq '.additions + .deletions')"
}
[ "$1" = required ] && { required_for_pr "$2"; exit 0; }

pr=$1
transcript=${2:-}

required=$(required_for_pr "$pr")
if [ -z "$required" ]; then
	gh pr edit "$pr" --add-label fix:reviewed
	exit 0
fi

if [ ! -s "$transcript" ]; then
	gh pr edit "$pr" --remove-label fix:reviewed >/dev/null 2>&1 || true
	gh pr comment "$pr" --body "@$FIXER_NOTIFY the fixer run left no transcript, so its reviews cannot be checked and this stays a draft: $FIXER_RUN_URL. Add fix:reviewed to let it go ready once CI is green."
	exit 0
fi

ran=$(jq -r -f .github/actions/refused-calls/tool-calls.jq "$transcript" | "$gate" fanned-out)
echo "Skills that ran their agents: $(paste -sd' ' <<<"${ran:-none}")" >>"${GITHUB_STEP_SUMMARY:-/dev/stderr}"

missing=''
for want in $required; do
	grep -qx "$want" <<<"$ran" || missing="$missing $want"
done

# The skills' agents can run and the agent still stop before weighing their
# findings; the Review section it writes last is the sign it got through.
if [ -z "$missing" ] && ! gh pr view "$pr" --json body --jq .body | grep -q '^## Review'; then
	gh pr comment "$pr" --body "@$FIXER_NOTIFY the fixer run ended without writing its Review section, so this stays a draft: $FIXER_RUN_URL. Add fix:reviewed to let it go ready once CI is green."
	exit 0
fi

if [ -z "$missing" ]; then
	gh pr edit "$pr" --add-label fix:reviewed
else
	gh pr edit "$pr" --remove-label fix:reviewed >/dev/null 2>&1 || true
	gh pr comment "$pr" --body "@$FIXER_NOTIFY the fixer run did not run these reviews through to their agents:${missing}. This stays a draft: $FIXER_RUN_URL. Add fix:reviewed to let it go ready once CI is green."
fi
