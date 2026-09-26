#!/usr/bin/env bash
# Prints the logins on stdin that can write to this repository. author_association
# is not used: to the workflow token, a private org member reads as a contributor.
set -uo pipefail

sort -u | while read -r login; do
	[ -n "$login" ] || continue
	case "$(gh api "repos/$GITHUB_REPOSITORY/collaborators/$login/permission" --jq .permission 2>/dev/null)" in
	admin | maintain | write) echo "$login" ;;
	esac
done
