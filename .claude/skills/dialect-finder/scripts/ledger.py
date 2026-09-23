#!/usr/bin/env python3
"""Arithmetic over the finder ledger, so coverage and the next positions to try
are computed rather than estimated by the agent.

    ledger.py status              coverage per layer, quiet streak, current layer
    ledger.py next LAYER COUNT    positions that cover the most untried pairs
    ledger.py dedupe FILE         drop candidate cases whose SQL was already run
    ledger.py append TABLE FILE   validate rows and append them to TABLE.jsonl

The ledger directory is $FINDER_MEMORY, default .finder-memory. The layout is
in ../references/ledger.md.
"""

import hashlib
import itertools
import json
import os
import random
import sys

LAYERS = ["permission", "completion", "lint", "resolution"]
DIALECTS = ["postgresql", "mysql", "sqlite"]
MEMORY = os.environ.get("FINDER_MEMORY", ".finder-memory")
QUIET_RUNS = int(os.environ.get("FINDER_QUIET_RUNS", "5"))
# Past this many cells the product is sampled rather than enumerated.
PRODUCT_ENUMERATION_LIMIT = 2_000_000


def read_jsonl(name):
    path = os.path.join(MEMORY, name)
    if not os.path.exists(path):
        return []
    with open(path, encoding="utf-8") as f:
        return [json.loads(line) for line in f if line.strip()]


def read_grid():
    path = os.path.join(MEMORY, "grid.json")
    if not os.path.exists(path):
        return {"version": 0, "phase": "layered", "layers": {}}
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def sql_hash(sql):
    normalized = " ".join(sql.lower().split())
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()[:16]


def pairs_of(position, dimensions):
    """Every (dimension, value) pair the position covers, on the current grid."""
    items = sorted(
        (dim, value) for dim, value in position.items()
        if dim in dimensions and value in dimensions[dim]
    )
    return {frozenset(pair) for pair in itertools.combinations(items, 2)}


def position_key(position, dimensions):
    return tuple((dim, position.get(dim)) for dim in sorted(dimensions))


class Layer:
    def __init__(self, name, dimensions, cases):
        self.name = name
        self.dimensions = dimensions
        self.impossible = set()
        dialects_by_position = {}
        positions = {}
        for case in cases:
            if case.get("layer") != name:
                continue
            if case.get("outcome") == "impossible":
                self.impossible |= pairs_of(case.get("impossible", {}), dimensions)
                continue
            key = position_key(case.get("position", {}), dimensions)
            if any(value is None for _, value in key):
                continue
            dialects_by_position.setdefault(key, set()).add(case.get("dialect"))
            positions[key] = case["position"]
        # A position counts once every dialect has a row for it, a skip included.
        self.tried = {
            key: positions[key] for key, dialects in dialects_by_position.items()
            if set(DIALECTS) <= dialects
        }
        self.covered = set(self.impossible)
        for position in self.tried.values():
            self.covered |= pairs_of(position, dimensions)

    def all_pairs(self):
        pairs = set()
        for (dim_a, values_a), (dim_b, values_b) in itertools.combinations(sorted(self.dimensions.items()), 2):
            for value_a in values_a:
                for value_b in values_b:
                    pairs.add(frozenset({(dim_a, value_a), (dim_b, value_b)}))
        return pairs

    def is_possible(self, position):
        return not (pairs_of(position, self.dimensions) & self.impossible)

    def product_size(self):
        if not self.dimensions:
            return 0
        size = 1
        for values in self.dimensions.values():
            size *= len(values)
        return size

    def cells(self):
        dims = sorted(self.dimensions)
        for values in itertools.product(*(self.dimensions[d] for d in dims)):
            yield dict(zip(dims, values))

    def product_coverage(self):
        """Tried positions over possible ones. Sampled when the grid is too big."""
        size = self.product_size()
        if size == 0:
            return 0, 0
        if size <= PRODUCT_ENUMERATION_LIMIT:
            possible = sum(1 for cell in self.cells() if self.is_possible(cell))
        else:
            rng = random.Random(0)
            sample = [self.random_position(rng) for _ in range(20_000)]
            possible = round(size * sum(map(self.is_possible, sample)) / len(sample))
        return len(self.tried), possible

    def random_position(self, rng):
        return {dim: rng.choice(values) for dim, values in self.dimensions.items()}


def quiet_streak(runs, layer):
    streak = 0
    for run in reversed([r for r in runs if r.get("layer") == layer]):
        if run.get("new_findings", 0):
            break
        streak += 1
    return streak


def status():
    grid = read_grid()
    cases = read_jsonl("cases.jsonl")
    runs = read_jsonl("runs.jsonl")
    report = {"grid_version": grid.get("version", 0), "phase": grid.get("phase", "layered"),
              "quiet_runs_needed": QUIET_RUNS, "layers": {}}
    for name in LAYERS:
        dimensions = grid.get("layers", {}).get(name, {}).get("dimensions", {})
        layer = Layer(name, dimensions, cases)
        all_pairs = layer.all_pairs()
        pairs_covered = len(all_pairs & layer.covered)
        product_tried, product_possible = layer.product_coverage()
        streak = quiet_streak(runs, name)
        report["layers"][name] = {
            "has_grid": bool(dimensions),
            "cases": sum(1 for c in cases if c.get("layer") == name),
            "pairs": f"{pairs_covered}/{len(all_pairs)}",
            "product": f"{product_tried}/{product_possible}",
            "quiet_streak": streak,
            "done": bool(dimensions) and pairs_covered == len(all_pairs) and streak >= QUIET_RUNS,
            "product_ratio": product_tried / product_possible if product_possible else 0.0,
        }

    layers = report["layers"]
    if report["phase"] == "rotating":
        report["current_layer"] = min(LAYERS, key=lambda n: layers[n]["product_ratio"])
    else:
        report["current_layer"] = next((n for n in LAYERS if not layers[n]["done"]), None)
    report["propose_rotation"] = report["phase"] == "layered" and report["current_layer"] is None
    json.dump(report, sys.stdout, indent=2)
    print()


def next_positions(layer_name, count, seed):
    grid = read_grid()
    dimensions = grid.get("layers", {}).get(layer_name, {}).get("dimensions", {})
    if not dimensions:
        sys.exit(f"no grid for layer {layer_name!r}; define its dimensions in grid.json first")
    layer = Layer(layer_name, dimensions, read_jsonl("cases.jsonl"))
    rng = random.Random(seed)
    uncovered = layer.all_pairs() - layer.covered
    tried = set(layer.tried)
    chosen = []

    def acceptable(position):
        key = position_key(position, dimensions)
        return key not in tried and layer.is_possible(position)

    # Pairwise first: each pick is the candidate covering the most untried pairs.
    while uncovered and len(chosen) < count:
        candidates = []
        for pair in rng.sample(sorted(uncovered, key=sorted), min(len(uncovered), 200)):
            position = layer.random_position(rng)
            position.update(dict(pair))
            candidates.append(position)
        candidates += [layer.random_position(rng) for _ in range(200)]
        candidates = [p for p in candidates if acceptable(p)]
        if not candidates:
            break
        best = max(candidates, key=lambda p: len(pairs_of(p, dimensions) & uncovered))
        if not pairs_of(best, dimensions) & uncovered:
            break
        chosen.append(best)
        tried.add(position_key(best, dimensions))
        uncovered -= pairs_of(best, dimensions)

    # Then the rest of the product, at random, so the full grid keeps filling.
    attempts = 0
    while len(chosen) < count and attempts < count * 200:
        attempts += 1
        position = layer.random_position(rng)
        if acceptable(position):
            chosen.append(position)
            tried.add(position_key(position, dimensions))

    for position in chosen:
        print(json.dumps({"layer": layer_name, "grid_version": grid.get("version", 0), "position": position}))
    print(f"{len(chosen)} positions, {len(uncovered)} pairs still untried after them", file=sys.stderr)


def dedupe(path):
    # The caret stays in the hashed text: the same SQL completed at another
    # position is another case.
    seen = {(c.get("sql_hash"), c.get("dialect")) for c in read_jsonl("cases.jsonl")}
    kept = dropped = 0
    with open(path, encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            case = json.loads(line)
            case["sql_hash"] = sql_hash(case["sql"])
            key = (case["sql_hash"], case.get("dialect"))
            if key in seen:
                dropped += 1
                continue
            seen.add(key)
            kept += 1
            print(json.dumps(case))
    print(f"kept {kept}, dropped {dropped} already run", file=sys.stderr)


REQUIRED = {
    "cases": {"id", "run", "layer", "dialect", "outcome"},
    "findings": {"id", "run", "layer", "group_key", "severity", "oracle", "status"},
    "precedents": {"id", "issue", "rule"},
    "runs": {"run", "date", "layer", "new_findings"},
}


def append(table, path):
    """Appends only when every row is valid, so a bad row never lands half a batch."""
    if table not in REQUIRED:
        sys.exit(f"unknown table {table!r}; want one of {sorted(REQUIRED)}")
    rows = []
    with open(path, encoding="utf-8") as f:
        for number, line in enumerate(f, 1):
            if not line.strip():
                continue
            row = json.loads(line)
            missing = REQUIRED[table] - row.keys()
            if missing:
                sys.exit(f"{path}:{number}: missing {sorted(missing)}")
            if table == "cases" and row["outcome"] not in {"pass", "mismatch", "skipped", "impossible"}:
                sys.exit(f"{path}:{number}: outcome {row['outcome']!r}")
            rows.append(row)
    with open(os.path.join(MEMORY, f"{table}.jsonl"), "a", encoding="utf-8") as f:
        for row in rows:
            f.write(json.dumps(row) + "\n")
    print(f"appended {len(rows)} rows to {table}.jsonl", file=sys.stderr)


def main(argv):
    if len(argv) >= 2 and argv[1] == "status":
        status()
    elif len(argv) >= 4 and argv[1] == "next":
        seed = argv[4] if len(argv) >= 5 else os.environ.get("GITHUB_RUN_ID", "0")
        next_positions(argv[2], int(argv[3]), seed)
    elif len(argv) >= 3 and argv[1] == "dedupe":
        dedupe(argv[2])
    elif len(argv) >= 4 and argv[1] == "append":
        append(argv[2], argv[3])
    else:
        sys.exit(__doc__)


if __name__ == "__main__":
    main(sys.argv)
