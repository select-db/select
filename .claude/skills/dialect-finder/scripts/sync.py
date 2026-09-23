#!/usr/bin/env python3
"""Brings the ledger up to date with what happened to the issues the finder
filed. Runs in the workflow before the agent, so the agent reads statuses and
precedents rather than issue threads.

A verdict counts only when the verdict: label was added by someone with write
access. Anyone else's label is ignored, so a passer-by cannot teach the finder
that a bypass is intended.
"""

import json
import os
import subprocess
import sys

MEMORY = os.environ.get("FINDER_MEMORY", ".finder-memory")
REPO = os.environ["GITHUB_REPOSITORY"]
RUN = os.environ.get("GITHUB_RUN_ID", "local")
TRUSTED = {"admin", "maintain", "write"}
STATUS_BY_VERDICT = {
    "verdict:fixed": "fixed",
    "verdict:not-a-bug": "rejected",
    "verdict:duplicate": "duplicate",
    "verdict:wontfix": "wontfix",
}
RULE_LIMIT = 300


def gh_json(*args):
    out = subprocess.run(["gh", *args], check=True, capture_output=True, text=True).stdout
    return json.loads(out) if out.strip() else None


def read_jsonl(name):
    path = os.path.join(MEMORY, name)
    if not os.path.exists(path):
        return []
    with open(path, encoding="utf-8") as f:
        return [json.loads(line) for line in f if line.strip()]


def append_jsonl(name, rows):
    if not rows:
        return
    with open(os.path.join(MEMORY, name), "a", encoding="utf-8") as f:
        for row in rows:
            f.write(json.dumps(row) + "\n")


_permissions = {}


def can_write(login):
    if login not in _permissions:
        try:
            answer = gh_json("api", f"repos/{REPO}/collaborators/{login}/permission")
            _permissions[login] = (answer or {}).get("permission") in TRUSTED
        except subprocess.CalledProcessError:
            _permissions[login] = False
    return _permissions[login]


def trusted_verdict(number):
    """The last verdict label a writer added, and who added it."""
    events = gh_json("api", "--paginate", "--slurp", f"repos/{REPO}/issues/{number}/events") or []
    verdict = None
    for page in events:
        for event in page:
            if event.get("event") != "labeled":
                continue
            name = event.get("label", {}).get("name", "")
            login = (event.get("actor") or {}).get("login", "")
            if name in STATUS_BY_VERDICT and can_write(login):
                verdict = (name, login)
    return verdict


def closing_rule(issue, login):
    """First line of the last comment the ruling maintainer wrote, quoted as data."""
    for comment in reversed(issue.get("comments", [])):
        if (comment.get("author") or {}).get("login") != login:
            continue
        for line in comment.get("body", "").splitlines():
            line = line.strip()
            if line:
                return line[:RULE_LIMIT]
    return ""


def main():
    latest = {}
    for row in read_jsonl("findings.jsonl"):
        latest[row["id"]] = row
    known_precedents = {row.get("issue") for row in read_jsonl("precedents.jsonl")}

    finding_rows, precedent_rows = [], []
    for finding in latest.values():
        number = finding.get("issue")
        if not number or finding.get("status") not in {"filed", "fixed", "rejected", "duplicate", "wontfix"}:
            continue
        issue = gh_json("issue", "view", str(number), "--repo", REPO, "--json", "state,comments")
        if issue["state"] == "OPEN":
            if finding["status"] != "filed":
                finding_rows.append({**finding, "run": RUN, "status": "filed", "reopened": True})
            continue
        verdict = trusted_verdict(number)
        if verdict is None:
            continue
        label, login = verdict
        status = STATUS_BY_VERDICT[label]
        if status == finding["status"]:
            continue
        finding_rows.append({**finding, "run": RUN, "status": status, "verdict_by": login})
        if status == "rejected" and number not in known_precedents:
            precedent_rows.append({
                "id": f"p-{number}",
                "issue": number,
                "layer": finding.get("layer"),
                "dialects": finding.get("dialects", []),
                "group_key": finding.get("group_key"),
                "rule": closing_rule(issue, login),
                "by": login,
            })

    append_jsonl("findings.jsonl", finding_rows)
    append_jsonl("precedents.jsonl", precedent_rows)
    print(f"synced: {len(finding_rows)} status changes, {len(precedent_rows)} new precedents", file=sys.stderr)


if __name__ == "__main__":
    main()
