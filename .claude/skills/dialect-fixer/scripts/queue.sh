#!/usr/bin/env bash
# The dialect fixer queue, run by dialect-fixer-queue.yml. Marks a green fixer
# pull request ready and requests $FIXER_REVIEWER, sends a conflicted or red one
# back to the fixer, and fills the free slots with the most severe open finder
# issues. At most $FIXER_LIMIT are in flight and one per area, since fixes in
# one area edit the same files. Prints the runs to start: [{issue, mode}].
set -euo pipefail

limit=${FIXER_LIMIT:-3}
max_repairs=3

# held: a run is on it, it waits on a person, or a person runs it by hand.
issues=$(gh issue list --label agent:finder --state open --limit 1000 --json number,labels --jq '
	map([.labels[].name] as $l | {number,
		area: ([$l[] | select(startswith("area:"))] | first // "area:none"),
		running: ($l | any(IN("fix:running", "fix:go"))),
		held: ($l | any(IN("fix:running", "fix:go", "fix:blocked", "fix:skip"))),
		# The order of the sev: labels in labels.sh.
		rank: ([$l[] as $x | ["sev:bypass", "sev:wrong-right", "sev:unchecked-read", "sev:false-denial", "sev:quality"] | index($x) | values] | min // 99)})')
# Every open claude/fix-N pull request, labelled or not, holds its issue's slot.
prs=$(gh pr list --state open --limit 100 --json number,headRefName,isDraft,mergeable,body,statusCheckRollup --jq '
	map(select(.headRefName | test("^claude/fix-[0-9]+$")) | {number, isDraft, mergeable,
		issue: (.headRefName | ltrimstr("claude/fix-") | tonumber),
		# dialect-fixer.yml appends one per repair run.
		repairs: (.body | [scan("<!-- fixer-repair -->")] | length),
		ci: ([.statusCheckRollup[] | select(.name == "CI OK") | .conclusion] | first // "")})')

runs='[]'
start() {
	gh issue edit "$1" --add-label fix:running >/dev/null
	runs=$(jq -c --argjson n "$1" --arg m "$2" '. + [{issue: $n, mode: $m}]' <<<"$runs")
}

while IFS=$'\t' read -r number issue draft mergeable ci repairs; do
	if [ "$mergeable" = CONFLICTING ] || [ "$ci" = FAILURE ]; then
		if [ "$repairs" -ge "$max_repairs" ]; then
			gh pr comment "$number" --body "The fixer stopped after $max_repairs repairs; it needs a person. Remove \`fix:blocked\` from #$issue to give it $max_repairs more." >/dev/null
			gh issue edit "$issue" --add-label fix:blocked >/dev/null
		else
			start "$issue" repair
		fi
	elif [ "$ci" = SUCCESS ] && [ "$mergeable" = MERGEABLE ] && [ "$draft" = true ]; then
		gh pr ready "$number" >/dev/null
		gh pr edit "$number" --add-reviewer "$FIXER_REVIEWER" >/dev/null
	fi
done < <(jq -r --argjson issues "$issues" '.[] | .issue as $n
	| select($issues | any(.number == $n and (.held | not)))
	| [.number, .issue, .isDraft, .mergeable, .ci, .repairs] | @tsv' <<<"$prs")

# The areas in flight: an open fixer pull request, or a run still working.
busy=$(jq -c --argjson prs "$prs" '[.[] | select(.running or (.number | IN($prs[].issue))) | .area]' <<<"$issues")

while read -r number area; do
	[ "$(jq length <<<"$busy")" -lt "$limit" ] || break
	jq -e --arg a "$area" 'index($a)' <<<"$busy" >/dev/null && continue
	start "$number" fix
	busy=$(jq -c --arg a "$area" '. + [$a]' <<<"$busy")
done < <(jq -r --argjson prs "$prs" '
	map(select((.held | not) and (.number | IN($prs[].issue) | not)))
	| sort_by(.rank, .number)[] | "\(.number) \(.area)"' <<<"$issues")

echo "$runs"
