"""Tests for file_level.py, and for the property that makes it a module."""
from sqlglot.dialects.dialect import Dialect

from analysis import analyze

SCHEMA = {"main": {"t1": {"c1": "INTEGER", "c2": "TEXT"}}}


def _r(sql, dialect="postgresql"):
    return analyze(sql, dialect=dialect, schema_dict=SCHEMA, default_schema="main")


def _diags(r, rule_id):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


def _statements(n):
    return ";\n".join(f"SELECT c1, c2 FROM t1 WHERE c1 > {i}" for i in range(n)) + ";"


def _scans_during(sql):
    """How many times a whole-file scan of the source happens in one lint."""
    real = Dialect.tokenize
    count = 0

    def counting(self, text, *args, **kwargs):
        nonlocal count
        count += 1
        return real(self, text, *args, **kwargs)

    Dialect.tokenize = counting
    try:
        _r(sql)
    finally:
        Dialect.tokenize = real
    return count


class TestWholeFileRulesRunOnce:
    """
    A rule that scans the source costs the same whatever statement it is handed,
    so running it per statement makes a lint pass quadratic. Measured on a file
    of 100 statements, one such rule was 76% of the run and the copies it made
    were discarded by the diagnostic dedupe, which is why it read as correct.
    """

    def test_scanning_does_not_grow_with_statement_count(self):
        one = _scans_during(_statements(1))
        many = _scans_during(_statements(20))
        assert one == many, (
            f"1 statement scanned the file {one} times, 20 statements scanned it "
            f"{many}: a whole-file rule is running inside the per-statement loop"
        )


class TestF001TrailingComma:
    def test_before_from(self):
        assert len(_diags(_r("SELECT c1, FROM t1"), "trailing-comma")) == 1

    def test_before_closing_paren(self):
        assert len(_diags(_r("SELECT c1 FROM t1 WHERE c1 IN (1, 2,)"), "trailing-comma")) == 1

    def test_no_trigger_on_a_well_formed_list(self):
        assert _diags(_r("SELECT c1, c2 FROM t1"), "trailing-comma") == []

    def test_reported_once_per_file_not_once_per_statement(self):
        """The dedupe used to hide the duplicates; nothing should produce them."""
        sql = "SELECT c1, FROM t1;\nSELECT c2 FROM t1;\nSELECT c1 FROM t1;"
        assert len(_diags(_r(sql), "trailing-comma")) == 1

    def test_found_in_a_later_statement(self):
        sql = "SELECT c1 FROM t1;\nSELECT c1, FROM t1;"
        diags = _diags(_r(sql), "trailing-comma")
        assert len(diags) == 1
        assert diags[0]["start_line"] == 2

    def test_reads_the_dialect_own_quoting(self):
        """MySQL backticks tokenize only under the MySQL tokenizer."""
        assert len(_diags(_r("SELECT `c1`, FROM t1", "mysql"), "trailing-comma")) == 1
