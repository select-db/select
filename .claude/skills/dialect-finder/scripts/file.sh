#!/usr/bin/env bash
# Files the issues the finder left in .finder-work/file/*.md, at most
# $FINDER_MAX_ISSUES. Each file: "title:", "labels:", optional "assign: yes",
# a blank line, then the body. Only finder labels pass, so no fix:go.
set -uo pipefail

label='(agent:finder|bug|needs-triage|(area|dialect|sev|oracle):[a-z-]+)'
filed=0 failed=0
for file in .finder-work/file/*.md; do
	[ -f "$file" ] || continue
	[ "$filed" -lt "${FINDER_MAX_ISSUES:-5}" ] || { echo "over the cap: $file" >&2; continue; }
	grep -q '^$' "$file" || { echo "skipped $file: no blank line after the headers" >&2; continue; }
	headers=$(sed '/^$/q' "$file")
	title=$(sed -n 's/^title: *//p' <<<"$headers" | sed -n 1p)
	labels=$(sed -n 's/^labels: *//p' <<<"$headers" | sed -n 1p)
	[ -n "$title" ] && [[ $labels =~ ^$label(,$label)*$ ]] ||
		{ echo "skipped $file: needs a title: line and finder labels" >&2; continue; }
	[[ ,$labels, == *,agent:finder,* ]] || labels="agent:finder,$labels"
	body=$(mktemp)
	sed '1,/^$/d' "$file" >"$body"
	args=(--title "$title" --label "$labels" --body-file "$body")
	grep -qx 'assign: yes' <<<"$headers" && args+=(--assignee "$FINDER_NOTIFY")
	gh issue create "${args[@]}" && filed=$((filed + 1)) || failed=1
done
exit "$failed"
