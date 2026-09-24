#!/usr/bin/env bash
# Files what the finder left in .finder-work/file/, after its run and with the
# workflow's token; the agent itself never talks to GitHub.
#   <finding id>.md    a new issue: "title:", "labels:" and optional "assign: yes"
#                      lines, a blank line, then the body. Its number goes into
#                      the finding's ledger row.
#   comment-<n>.md     a comment on issue n, which must be a finder issue.
set -euo pipefail

dir=.finder-work/file
memory=${FINDER_MEMORY:-.finder-memory}
[ -d "$dir" ] || exit 0

header() { sed -n "1,/^\$/s/^$1: *//p" "$2" | head -n1; }

for file in "$dir"/*.md; do
	[ -f "$file" ] || continue
	name=$(basename "$file" .md)
	case "$name" in
	comment-[0-9]*)
		issue=${name#comment-}
		if gh issue view "$issue" --json labels --jq '.labels[].name' | grep -qx agent:finder; then
			gh issue comment "$issue" --body-file "$file"
		else
			echo "skipped $file: #$issue is not a finder issue" >&2
		fi
		;;
	*)
		title=$(header title "$file")
		labels=$(header labels "$file")
		[[ $labels =~ ^[a-z0-9:_-]+(,[a-z0-9:_-]+)*$ ]] && [ -n "$title" ] ||
			{ echo "skipped $file: needs title: and labels: lines" >&2; continue; }
		args=(--title "$title" --label "$labels" --body "$(sed '1,/^$/d' "$file")")
		[ "$(header assign "$file")" = yes ] && args+=(--assignee "$FINDER_NOTIFY")
		number=$(gh issue create "${args[@]}" | grep -oE '[0-9]+$')
		echo "filed #$number from $file"
		# The finding's latest row again, now with its issue.
		jq -sc --arg id "$name" --argjson n "$number" \
			'map(select(.id == $id)) | last // empty | .issue = $n | .status = "filed"' \
			"$memory/findings.jsonl" | python3 .claude/skills/dialect-finder/scripts/ledger.py append findings /dev/stdin
		;;
	esac
done
