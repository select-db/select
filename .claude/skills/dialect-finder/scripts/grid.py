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
    return [json.loads(line) for p in map(Path, filter(None, paths)) if p.is_file()
            for line in p.read_text().splitlines() if line.strip()]


def pairs(position):
    return set(itertools.combinations(sorted(position.items()), 2))


def valid(layer, position):
    axes = LAYERS.get(layer, {})
    return bool(position) and all(v in axes.get(k, ()) for k, v in position.items())


def dead(layer, pair):
    """A pair a reviewed requires rule in grid.json forbids."""
    requires = GRID["layers"][layer].get("requires", {})
    for (a, x), (b, y) in (pair, pair[::-1]):
        allowed = requires.get(f"{a}={x}", {}).get(b)
        if allowed is not None and y not in allowed:
            return True
    return False


def coverage(layer, cases):
    items = [(axis, v) for axis, values in sorted(LAYERS[layer].items()) for v in values]
    possible = {p for p in itertools.combinations(items, 2) if p[0][0] != p[1][0] and not dead(layer, p)}
    tried = set()
    for r in cases:
        if r.get("layer") != layer:
            continue
        if r.get("outcome") == "impossible" and len(r.get("impossible", {})) == 2 and valid(layer, r["impossible"]):
            possible -= pairs(r["impossible"])
        elif r.get("outcome") in ("pass", "mismatch", "skipped") and valid(layer, r.get("position")):
            tried |= pairs(r["position"])
    return possible, tried


def next_positions(layer, n, cases):
    possible, tried = coverage(layer, cases)
    todo = possible - tried
    axes = sorted(LAYERS[layer].items())
    for _ in range(n):
        if not todo:
            return
        # Seed with one untried pair, then give each other axis the value that
        # covers the most untried pairs with the values already set.
        (a, x), (b, y) = min(todo)
        pos = {a: x, b: y}
        for axis, values in axes:
            if axis in pos:
                continue
            ok = [v for v in values if all(tuple(sorted(p)) in possible for p in (((axis, v), (k, w)) for k, w in pos.items()))]
            if not ok:
                break
            pos[axis] = max(ok, key=lambda v: sum(tuple(sorted(((axis, v), (k, w)))) in todo for k, w in pos.items()))
        todo.discard(((a, x), (b, y)))
        if len(pos) == len(axes):
            todo -= pairs(pos)
            print(json.dumps(pos))


if __name__ == "__main__":
    cases = rows()
    if sys.argv[1:2] == ["status"]:
        for layer in LAYERS:
            possible, tried = coverage(layer, cases)
            print(f"{layer}: {len(possible & tried)}/{len(possible)} pairs tried")
    elif sys.argv[1:2] == ["next"]:
        next_positions(sys.argv[2], int(sys.argv[3]), cases)
    else:
        sys.exit(__doc__)
