#!/usr/bin/env python3
"""Pairwise coverage of .claude/dialect/grid.json by the finder's case rows.

  grid.py status               pairs tried per layer
  grid.py next <layer> <n>     n positions covering the most untried pairs

Reads $FINDER_CASES and .finder-work/cases.jsonl. A row:
{"layer", "position": {axis: value}, "dialect", "sql", "outcome": pass|mismatch|skipped}
or {"layer", "outcome": "impossible", "impossible": {axis: value, axis: value}, "reason"}.
"""
import itertools
import json
import os
import sys
from pathlib import Path

GRID = json.loads((Path(__file__).resolve().parents[3] / "dialect/grid.json").read_text())
LAYERS = {
    name: {axis: GRID["shared"][v] if isinstance(v, str) else v for axis, v in layer["axes"].items()}
    for name, layer in GRID["layers"].items()
}


def rows():
    paths = [os.environ.get("FINDER_CASES", ""), ".finder-work/cases.jsonl"]
    out = []
    for p in map(Path, filter(None, paths)):
        for line in p.read_text().splitlines() if p.is_file() else ():
            try:
                row = json.loads(line)
            except ValueError:
                continue
            if isinstance(row, dict):
                out.append(row)
    return out


def pair(a, b):
    return (a, b) if a < b else (b, a)


def pairs(position):
    return set(itertools.combinations(sorted(position.items()), 2))


def valid(layer, position):
    axes = LAYERS.get(layer, {})
    return isinstance(position, dict) and bool(position) and all(v in axes.get(k, ()) for k, v in position.items())


def forbidden(layer, p):
    requires = {**GRID["shared"]["requires"], **GRID["layers"][layer].get("requires", {})}
    for (a, x), (b, y) in (p, p[::-1]):
        allowed = requires.get(f"{a}={x}", {}).get(b)
        if allowed is not None and y not in allowed:
            return True
    return False


def coverage(layer, cases):
    items = [(axis, v) for axis, values in sorted(LAYERS[layer].items()) for v in values]
    possible = {p for p in itertools.combinations(items, 2) if p[0][0] != p[1][0] and not forbidden(layer, p)}
    tried, ruled = set(), 0
    for r in cases:
        if r.get("layer") != layer:
            continue
        if r.get("outcome") == "impossible" and len(r.get("impossible", {})) == 2 and valid(layer, r["impossible"]):
            possible -= pairs(r["impossible"])
            ruled += 1
        elif r.get("outcome") in ("pass", "mismatch", "skipped") and valid(layer, r.get("position")):
            tried |= pairs(r["position"])
    return possible, tried, ruled


def next_positions(layer, n, cases):
    possible, tried, _ = coverage(layer, cases)
    todo = possible - tried
    axes = sorted(LAYERS[layer].items())
    while todo and n:
        # Greedy per axis, so each position covers many untried pairs at once.
        seed = min(todo)
        pos = dict(seed)
        for axis, values in axes:
            if axis in pos:
                continue
            allowed = [v for v in values if all(pair((axis, v), kw) in possible for kw in pos.items())]
            if not allowed:
                break
            pos[axis] = max(allowed, key=lambda v: sum(pair((axis, v), kw) in todo for kw in pos.items()))
        todo.discard(seed)
        if len(pos) == len(axes):
            todo -= pairs(pos)
            print(json.dumps(pos))
            n -= 1


if __name__ == "__main__":
    cases = rows()
    if sys.argv[1:2] == ["status"]:
        for layer in LAYERS:
            possible, tried, ruled = coverage(layer, cases)
            print(f"{layer}: {len(possible & tried)}/{len(possible)} pairs tried, {ruled} ruled impossible by the finder")
    elif sys.argv[1:2] == ["next"] and len(sys.argv) == 4 and sys.argv[2] in LAYERS:
        next_positions(sys.argv[2], int(sys.argv[3]), cases)
    else:
        sys.exit(__doc__)
