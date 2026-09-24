#!/usr/bin/env bash
# Files what the finder left in .finder-work/file/, with the workflow's token.
#   <finding id>.md  "title:", "labels:", optional "assign: yes", a blank line, the body
#   comment-<n>.md   a comment on issue n, which must be a finder issue
set -uo pipefail

dir=.finder-work/file
memory=${FINDER_MEMORY:-.finder-memory}
ledger=.claude/skills/dialect-finder/scripts/ledger.py
[ -d "$dir" ] || exit 0
failed=0

for file in "$dir"/*.md; do
	[ -f "$file" ] || continue
	name=$(basename "$file" .md)
	case "$name" in
	comment-[0-9]*)
		issue=${name#comment-}
		if gh issue view "$issue" --json labels --jq '.labels[].name' | grep -qx agent:finder; then
			gh issue comment "$issue" --body-file "$file" || failed=1
		else
			echo "skipped $file: #$issue is not a finder issue" >&2
		fi
		;;
	*)
		grep -q '^$' "$file" || { echo "skipped $file: no blank line after the headers" >&2; continue; }
		headers=$(sed '/^$/q' "$file")
		title=$(sed -n 's/^title: *//p' <<<"$headers" | sed -n 1p)
		labels=$(sed -n 's/^labels: *//p' <<<"$headers" | sed -n 1p)
		[ -n "$title" ] && [[ $labels =~ ^[a-z0-9:_-]+(,[a-z0-9:_-]+)*$ ]] ||
			{ echo "skipped $file: needs title: and labels: lines" >&2; continue; }
		body=$(mktemp)
		sed '1,/^$/d' "$file" >"$body"
		args=(--title "$title" --label "$labels" --body-file "$body")
		grep -qx 'assign: yes' <<<"$headers" && args+=(--assignee "$FINDER_NOTIFY")
		number=$(gh issue create "${args[@]}" | grep -oE '[0-9]+$') ||
			{ echo "failed to file $file" >&2; failed=1; continue; }
		echo "filed #$number from $file"
		row=$(jq -sc --arg id "$name" --argjson n "$number" \
			'map(select(.id == $id)) | last // empty | .issue = $n | .status = "filed"' "$memory/findings.jsonl")
		if [ -z "$row" ]; then
			echo "no ledger row for $name; #$number is not recorded" >&2
		else
			python3 "$ledger" append findings /dev/stdin <<<"$row" || failed=1
		fi
		;;
	esac
done
exit "$failed"
