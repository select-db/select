#!/usr/bin/env bash
# Report sqlc-generated queries that nothing calls.
#
# No other gate sees these: deadcode does not reach generated code, and sqlc
# emits whatever sits in the sql directory, so a query outlives its last caller
# and reads as live schema surface to whoever finds it next.
#
# Usage: .github/scripts/unused_queries.sh <module-dir>
set -euo pipefail

root="${1:?usage: unused_queries.sh <module-dir>}"
generated="$root/db/generated/queries.sql.go"
[ -f "$generated" ] || { echo "no generated queries at $generated" >&2; exit 1; }

unused=()
while read -r method; do
  # Fixed-string, because the receiver call is literally ".Method(".
  grep -rqF --include='*.go' --exclude-dir=generated -- ".$method(" "$root" ||
    unused+=("$method")
done < <(grep -oP '^func \(q \*Queries\) \K\w+' "$generated" | sort -u)

if [ ${#unused[@]} -gt 0 ]; then
  printf 'unused query: %s\n' "${unused[@]}"
  echo
  echo "Delete each one's .sql file and regenerate, or add the caller."
  exit 1
fi

echo "every generated query in $root has a caller"
