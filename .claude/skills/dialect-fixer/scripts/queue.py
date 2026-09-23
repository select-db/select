#!/usr/bin/env python3
"""The fixer's queue: open agent:finder issues a writer labelled fix:go, oldest
first, that no fixer run has taken yet.

    queue.py next     print the issue number to work on, or nothing
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "dialect-finder", "scripts"))
from sync import REPO, gh_json, trusted_label  # noqa: E402

TAKEN = {"fix:in-progress", "fix:pr-open", "fix:blocked"}


def queued():
    issues = gh_json("issue", "list", "--repo", REPO, "--state", "open",
                     "--label", "agent:finder", "--label", "fix:go",
                     "--limit", "200", "--json", "number,labels") or []
    numbers = []
    for issue in sorted(issues, key=lambda i: i["number"]):
        labels = {label["name"] for label in issue["labels"]}
        # Triage can label too; only a writer's fix:go spends the subscription.
        if not labels & TAKEN and trusted_label(issue["number"], {"fix:go"}):
            numbers.append(issue["number"])
    return numbers


def main(argv):
    if argv[1:] == ["next"]:
        numbers = queued()
        if numbers:
            print(numbers[0])
    else:
        sys.exit(__doc__)


if __name__ == "__main__":
    main(sys.argv)
