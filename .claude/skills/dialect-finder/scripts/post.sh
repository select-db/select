#!/usr/bin/env bash
# Posts outside the agent, so a run that crashed still reports.
#   post.sh log <summary.md>   comment the run summary on the run-log issue
#   post.sh failed <run url>   open or update the run-failed issue
# Both notify $FINDER_NOTIFY: the log by mention, the failure by assignment.
set -euo pipefail

mode=$1
notify=${FINDER_NOTIFY:-}
mention=${notify:+@$notify }

# The open agent:finder issue with exactly this title, or nothing.
find_issue() {
	gh issue list --state open --label agent:finder --limit 100 --json number,title \
		--jq ".[] | select(.title == \"$1\") | .number" | head -n1
}

open_or_comment() {
	local title=$1 body=$2 labels=$3 number
	number=$(find_issue "$title")
	if [ -n "$number" ]; then
		gh issue comment "$number" --body-file "$body" >/dev/null
		echo "commented on #$number"
		return
	fi
	gh issue create --title "$title" --body-file "$body" --label "$labels" \
		${notify:+--assignee "$notify"}
}

body=$(mktemp)
case "$mode" in
log)
	{ echo "${mention}run summary"; echo; cat "$2"; } >"$body"
	open_or_comment "Dialect finder: run log" "$body" agent:finder
	;;
failed)
	cat >"$body" <<MSG
${mention}the dialect finder run failed: $2

Whatever the ledger checkpointed before the failure is on \`agent/finder-memory\`.
The next scheduled run starts from there.
MSG
	open_or_comment "Dialect finder: run failed" "$body" agent:finder,needs-triage
	;;
*)
	echo "usage: post.sh log <summary.md> | post.sh failed <run url>" >&2
	exit 2
	;;
esac
