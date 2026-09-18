"""
Where a diagnostic points, for every rule at once.

A diagnostic the editor cannot draw is the same to a user as no diagnostic at
all, and it passes a test that only counts rule ids. Both failures here were
invisible that way: a range of zero width draws nothing, and a range at 1:0
draws on the first character of the file rather than on the clause.
"""
from analysis import analyze

SCHEMA = {"main": {
    "t1": {"c1": "INTEGER", "c2": "TEXT"},
    "t2": {"c1": "INTEGER", "c3": "TEXT"},
}}


def _diags(sql, dialect="postgresql"):
    return analyze(sql, dialect=dialect, schema_dict=SCHEMA,
                   default_schema="main")["diagnostics"]


def _underlined(sql, diag):
    """The source text a diagnostic covers, for a range on one line."""
    line = sql.splitlines()[diag["start_line"] - 1]
    return line[diag["start_col"]:diag["end_col"]]


# One statement per rule that had no usable position, plus the rules that
# already did, so a change to the shared helper cannot fix one by breaking
# another.
CORPUS = [
    "SELECT c1 FROM t1 OFFSET 10",
    "SELECT c1 FROM t1 LIMIT 5",
    "SELECT c1 FROM t1 WHERE c2 = NULL",
    "SELECT c1 FROM t1 WHERE c2 <> NULL",
    "SELECT c1 FROM t1 WHERE c1 IN (1, 2, NULL)",
    "SELECT c1 FROM t1 WHERE c1 NOT IN (1, NULL)",
    "SELECT c1 FROM t1 WHERE c1 IN ()",
    "SELECT c1 FROM t1 WHERE c1 IN (1, 1)",
    "SELECT c1 FROM t1 WHERE c1 BETWEEN 10 AND 1",
    "SELECT c1 FROM t1 WHERE 1 = 1",
    "SELECT c1 FROM t1 WHERE c1 > 5 AND c1 < 3",
    "SELECT c1 FROM t1 WHERE c1 / 0 > 1",
    "SELECT c1, c1 FROM t1",
    "SELECT COALESCE(c1) FROM t1",
    "SELECT c1 FROM t1 WHERE c2 LIKE '%a%'",
    "SELECT c1 FROM t1 WHERE c2 LIKE 'a'",
    "SELECT c1 FROM t1 WHERE COUNT(*) > 1",
    "SELECT count(*) FROM t1 HAVING count(*) > 1",
    "SELECT nope FROM t1",
    "SELECT c1 FROM nosuch",
    "SELECT nosuchfn(c1) FROM t1",
    "SELECT abs() FROM t1",
    "SELECT c1 FROM t1 a JOIN t2 b ON a.c1 = b.c1 WHERE c1 = 1",
    "WITH x AS (SELECT c1 FROM t1) SELECT 1",
    "SELECT c1 FROM (SELECT c1 FROM t1 ORDER BY c1) s",
    "SELECT c1 FROM t1 WHERE ROW_NUMBER() OVER () > 1",
    "SELECT C1 FROM T1",
    # Multi-line: the clause is not on the line the statement starts on.
    "SELECT c1\nFROM t1\nWHERE c1 > 5 AND c1 < 3",
]


class TestEveryDiagnosticPointsAtItsClause:
    def test_no_diagnostic_is_empty(self):
        """A range of zero width draws no squiggle, so the rule is silent."""
        empty = [
            (sql, d["rule_id"])
            for sql in CORPUS
            for d in _diags(sql)
            if (d["start_line"], d["start_col"]) == (d["end_line"], d["end_col"])
        ]
        assert empty == []

    def test_no_diagnostic_falls_back_to_the_start_of_the_file(self):
        """1:0 is where a rule lands when it could not find its own clause."""
        origin = [
            (sql, d["rule_id"])
            for sql in CORPUS
            for d in _diags(sql)
            # A statement really starting at 1:0 owns that position; the
            # fallback is recognisable because it is empty as well.
            if (d["start_line"], d["start_col"], d["end_col"]) == (1, 0, 0)
        ]
        assert origin == []

    def test_every_range_lies_inside_its_line(self):
        for sql in CORPUS:
            lines = sql.splitlines()
            for d in _diags(sql):
                assert 1 <= d["start_line"] <= len(lines), (sql, d)
                line = lines[d["start_line"] - 1]
                assert d["end_col"] <= len(line), (sql, d, line)

    def test_a_clause_below_the_first_line_is_reported_there(self):
        sql = "SELECT c1\nFROM t1\nWHERE c1 > 5 AND c1 < 3"
        diag = next(d for d in _diags(sql) if d["rule_id"] == "contradictory-predicate")
        assert diag["start_line"] == 3
        assert _underlined(sql, diag) == "c1 > 5 AND c1 < 3"


class TestWhatEachRuleUnderlines:
    """
    The text a reader sees marked. Pinned per rule, because "not empty" is
    satisfied by underlining the wrong token just as well as the right one.
    """

    CASES = [
        ("SELECT c1 FROM t1 OFFSET 10", "offset-without-limit", "10"),
        ("SELECT c1 FROM t1 LIMIT 5", "limit-without-order-by", "5"),
        ("SELECT c1 FROM t1 WHERE c2 = NULL", "null-equality", "c2"),
        ("SELECT c1 FROM t1 WHERE c2 <> NULL", "null-inequality", "c2"),
        ("SELECT c1 FROM t1 WHERE c1 IN ()", "empty-in-list", "c1"),
        ("SELECT c1 FROM t1 WHERE c1 BETWEEN 10 AND 1", "reversed-between", "10"),
        ("SELECT c1 FROM t1 WHERE 1 = 1", "tautological-predicate", "1"),
        ("SELECT c1 FROM t1 WHERE c1 > 5 AND c1 < 3", "contradictory-predicate", "c1 > 5 AND c1 < 3"),
        ("SELECT c1 FROM t1 WHERE c1 / 0 > 1", "division-by-zero", "c1 / 0"),
        ("SELECT COALESCE(c1) FROM t1", "coalesce-single-arg", "COALESCE"),
        ("SELECT count(*) FROM t1 HAVING count(*) > 1", "having-without-group-by", "count(*) > 1"),
        ("SELECT c1 FROM t1 WHERE ROW_NUMBER() OVER () > 1", "window-in-where", "ROW_NUMBER"),
        ("SELECT c1 FROM (SELECT c1 FROM t1 ORDER BY c1) s", "subquery-order-by", "c1"),
        ("SELECT nope FROM t1", "unknown-column", "nope"),
        ("SELECT c1 FROM nosuch", "unknown-table", "nosuch"),
        ("SELECT nosuchfn(c1) FROM t1", "unknown-function", "nosuchfn"),
        ("SELECT c1, c1 FROM t1", "duplicate-column", "c1"),
        ("SELECT c1 FROM t1 WHERE c2 LIKE '%a%'", "like-leading-wildcard", "'%a%'"),
        ("SELECT C1 FROM T1", "unquoted-uppercase", "C1"),
    ]

    def test_underlines(self):
        for sql, rule_id, want in self.CASES:
            diag = next((d for d in _diags(sql) if d["rule_id"] == rule_id), None)
            assert diag is not None, f"{rule_id} did not fire on {sql!r}"
            assert _underlined(sql, diag) == want, (
                f"{rule_id} on {sql!r} underlined "
                f"{_underlined(sql, diag)!r}, want {want!r}"
            )


class TestNullKeywordHasNoPositionOfItsOwn:
    """
    sqlglot records no position for the NULL keyword, so a rule about a NULL in
    a list is pointed at the list instead of nowhere.
    """

    def test_null_in_list_marks_the_list(self):
        sql = "SELECT c1 FROM t1 WHERE c1 IN (1, 2, NULL)"
        diag = next(d for d in _diags(sql) if d["rule_id"] == "null-in-list")
        assert _underlined(sql, diag).startswith("c1 IN (")

    def test_null_in_not_in_marks_the_list(self):
        sql = "SELECT c1 FROM t1 WHERE c1 NOT IN (1, NULL)"
        diag = next(d for d in _diags(sql) if d["rule_id"] == "null-in-not-in")
        assert _underlined(sql, diag).startswith("c1 NOT IN (")
