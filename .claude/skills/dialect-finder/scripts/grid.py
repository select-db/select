#!/usr/bin/env python3
"""Pairwise coverage of .claude/dialect/grid.json by the finder's case rows.

  grid.py status <cases.jsonl>...              pairs tried per layer
  grid.py next <layer> <n> <cases.jsonl>...     n positions covering the most untried pairs
  grid.py add <memory> <new rows> <run>         append the rows that fit the grid

A row: {"layer", "position": {axis: value}, "dialect", "sql", "outcome": pass|mismatch|skipped}
or {"layer", "outcome": "impossible", "impossible": {axis: value, axis: value}, "reason"}.
"""
import itertools
import json
import random
import sys
from pathlib import Path

GRID = json.loads((Path(__file__).resolve().parents[3] / "dialect/grid.json").read_text())


def rows(*paths):
    return [json.loads(line) for p in map(Path, paths) if p.exists() for line in p.read_text().splitlines() if line.strip()]


def pairs(position):
    items = sorted(position.items())
    return {(a, b) for a, b in itertools.combinations(items, 2)}


def valid(layer, position):
    axes = GRID.get(layer)
    return bool(axes) and bool(position) and all(axes.get(k) and v in axes[k] for k, v in position.items())


def all_pairs(layer):
    items = [(axis, v) for axis, values in sorted(GRID[layer].items()) for v in values]
    return {(a, b) for a, b in itertools.combinations(items, 2) if a[0] != b[0]}


def coverage(layer, cases):
    tried, impossible = set(), set()
    for r in cases:
        if r.get("layer") != layer:
            continue
        if r.get("outcome") == "impossible" and valid(layer, r.get("impossible", {})) and len(r["impossible"]) == 2:
            impossible |= pairs(r["impossible"])
        elif r.get("outcome") in ("pass", "mismatch", "skipped") and valid(layer, r.get("position", {})):
            tried |= pairs(r["position"])
    return all_pairs(layer) - impossible, tried, impossible


def status(cases):
    for layer in GRID:
        possible, tried, impossible = coverage(layer, cases)
        print(f"{layer}: {len(possible & tried)}/{len(possible)} pairs tried, {len(impossible)} impossible")


def next_positions(cases, layer, n):
    possible, tried, impossible = coverage(layer, cases)
    todo = possible - tried
    rng = random.Random(len(cases))
    axes = GRID[layer]
    chosen = []
    for _ in range(n):
        if not todo:
            break
        # Seed each position with one untried pair, then fill the other axes greedily.
        (a, x), (b, y) = min(todo)
        best, gain = None, -1
        for _ in range(200):
            pos = {k: rng.choice(v) for k, v in axes.items()}
            pos[a], pos[b] = x, y
            ps = pairs(pos)
            if ps & impossible:
                continue
            if len(ps & todo) > gain:
                best, gain = pos, len(ps & todo)
        if best is None:
            todo.discard(((a, x), (b, y)))
            continue
        chosen.append(best)
        todo -= pairs(best)
    for pos in chosen:
        print(json.dumps(pos))


def add(path, new, run):
    kept = 0
    with open(path, "a") as out:
        for i, r in enumerate(rows(new)):
            ok = valid(r.get("layer"), r.get("impossible") if r.get("outcome") == "impossible" else r.get("position"))
            if not ok or r.get("outcome") not in ("pass", "mismatch", "skipped", "impossible"):
                print(f"rejected row {i + 1}: {json.dumps(r)[:200]}", file=sys.stderr)
                continue
            out.write(json.dumps({**r, "run": run}) + "\n")
            kept += 1
    print(f"added {kept} rows")


if __name__ == "__main__":
    cmd, args = sys.argv[1], sys.argv[2:]
    if cmd == "status":
        status(rows(*args))
    elif cmd == "next":
        next_positions(rows(*args[2:]), args[0], int(args[1]))
    elif cmd == "add":
        add(args[0], args[1], args[2])
    else:
        sys.exit(__doc__)
