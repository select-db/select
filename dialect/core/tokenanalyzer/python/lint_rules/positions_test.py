"""
What each rule marks in the source.

That a range can be drawn at all is checked for the whole suite in conftest.
Here is the part that is per rule: a range of the right width in the wrong
place passes that check just as well as the right one.
"""
from analysis import analyze

SCHEMA = {"main": {
    "t1": {"c1": "INTEGER", "c2": "TEXT"},
    "t2": {"c1": "INTEGER", "c3": "TEXT"},
}}


def _diags(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA,
                   default_schema="main")["diagnostics"]


def _marked(sql, diag):
    """The source text a diagnostic covers, across lines if it spans them."""
    lines = sql.splitlines()
    first, last = diag["start_line"] - 1, diag["end_line"] - 1
    if first == last:
        return lines[first][diag["start_col"]:diag["end_col"]]
    return "\n".join(
        [lines[first][diag["start_col"]:]]
        + lines[first + 1:last]
        + [lines[last][:diag["end_col"]]]
    )


def _mark(sql, rule_id):
    diag = next((d for d in _diags(sql) if d["rule_id"] == rule_id), None)
    assert diag is not None, f"{rule_id} did not fire on {sql!r}"
    return _marked(sql, diag)


# Every rule whose position this change moved, plus the ones that were already
# right, so a change to the shared helper cannot fix one by breaking another.
CASES = [
    ("SELECT c1 FROM t1 OFFSET 10", "offset-without-limit", "10"),
    ("SELECT c1 FROM t1 LIMIT 5", "limit-without-order-by", "5"),
    ("SELECT c1 FROM t1 WHERE c2 = NULL", "null-equality", "c2"),
    ("SELECT c1 FROM t1 WHERE c2 <> NULL", "null-inequality", "c2"),
    ("SELECT c1 FROM t1 WHERE c1 IN ()", "empty-in-list", "c1"),
    ("SELECT c1 FROM t1 WHERE c1 IN (1, 1)", "duplicate-in-value", "1"),
    ("SELECT c1 FROM t1 WHERE c1 BETWEEN 10 AND 1", "reversed-between", "10"),
    ("SELECT c1 FROM t1 WHERE 1 = 1", "tautological-predicate", "1"),
    ("SELECT c1 FROM t1 WHERE c1 > 5 AND c1 < 3", "contradictory-predicate", "c1 > 5 AND c1 < 3"),
    ("SELECT c1 FROM t1 WHERE c1 / 0 > 1", "division-by-zero", "c1 / 0"),
    ("SELECT COALESCE(c1) FROM t1", "coalesce-single-arg", "COALESCE"),
    ("SELECT count(*) FROM t1 HAVING count(*) > 1", "having-without-group-by", "count(*) > 1"),
    ("SELECT c1 FROM t1 WHERE ROW_NUMBER() OVER () > 1", "window-in-where", "ROW_NUMBER"),
    ("SELECT c1 FROM (SELECT c1 FROM t1 ORDER BY c1) s", "subquery-order-by", "c1"),
    ("SELECT c1 FROM t1 WHERE COUNT(*) > 1", "agg-in-where", "COUNT"),
    ("SELECT nope FROM t1", "unknown-column", "nope"),
    ("SELECT c1 FROM nosuch", "unknown-table", "nosuch"),
    ("SELECT nosuchfn(c1) FROM t1", "unknown-function", "nosuchfn"),
    ("SELECT abs() FROM t1", "missing-argument", "abs"),
    ("SELECT c1, c1 FROM t1", "duplicate-column", "c1"),
    ("SELECT c1, FROM t1", "trailing-comma", ","),
    ("SELECT c1 FROM t1 WHERE c2 LIKE '%a%'", "like-leading-wildcard", "'%a%'"),
    ("SELECT c1 FROM t1 WHERE c2 LIKE 'a'", "like-no-wildcard", "'a'"),
    ("SELECT C1 FROM T1", "unquoted-uppercase", "C1"),
    ("WITH x AS (SELECT c1 FROM t1) SELECT 1", "unused-cte", "x"),
    ("SELECT c1 FROM t1 a JOIN t2 b ON a.c1 = b.c1 WHERE c1 = 1", "ambiguous-column", "c1"),
]


def test_each_rule_marks_its_own_clause():
    for sql, rule_id, want in CASES:
        got = _mark(sql, rule_id)
        assert got == want, f"{rule_id} on {sql!r} marked {got!r}, want {want!r}"


class TestRangesThatCrossLines:
    """
    A clause is not always on the line its statement starts on, and not always
    on one line. The range carries its own end line for both.
    """

    def test_clause_below_the_first_line(self):
        sql = "SELECT c1\nFROM t1\nWHERE c1 / 0 > 1"
        assert _mark(sql, "division-by-zero") == "c1 / 0"

    def test_clause_spanning_two_lines(self):
        sql = "SELECT c1 FROM t1\nWHERE c1 > 5\n  AND c1 < 3"
        assert _mark(sql, "contradictory-predicate") == "c1 > 5\n  AND c1 < 3"


class TestNullKeywordHasNoPositionOfItsOwn:
    """
    sqlglot records nothing for the NULL keyword and it has no operands, so the
    rules about a NULL in a list mark the list instead of marking nowhere. The
    mark stops at the last token sqlglot did tag, which is why these are
    prefixes rather than the whole list.
    """

    CASES = [
        ("SELECT c1 FROM t1 WHERE c1 IN (1, 2, NULL)", "null-in-list", "c1 IN ("),
        ("SELECT c1 FROM t1 WHERE c1 NOT IN (1, NULL)", "null-in-not-in", "c1 NOT IN ("),
    ]

    def test_marks_the_list(self):
        for sql, rule_id, prefix in self.CASES:
            assert _mark(sql, rule_id).startswith(prefix)
