"""Tests for the shared dialect resolution and tokenizer helpers."""
import pytest

from analysis.schema import sqlglot_dialect_name, tokenize


class TestSqlglotDialectName:
    def test_maps_go_names(self):
        assert sqlglot_dialect_name("postgresql") == "postgres"
        assert sqlglot_dialect_name("mysql") == "mysql"
        assert sqlglot_dialect_name("sqlite") == "sqlite"

    def test_unknown_name_passes_through_so_sqlglot_raises(self):
        assert sqlglot_dialect_name("nosuchdb") == "nosuchdb"
        with pytest.raises(Exception):
            tokenize("SELECT 1", sqlglot_dialect_name("nosuchdb"))


class TestTokenize:
    def test_requires_a_dialect(self):
        with pytest.raises(ValueError):
            tokenize("SELECT 1", "")

    @pytest.mark.parametrize("sql,sg_dialect", [
        ("SELECT `c` FROM t", "mysql"),
        ("SELECT [c] FROM t", "sqlite"),
        ('SELECT "c" FROM t', "postgres"),
    ])
    def test_quoted_identifier_is_one_identifier_token(self, sql, sg_dialect):
        texts = [t.text for t in tokenize(sql, sg_dialect)]
        assert "c" in texts
        assert "`" not in texts and "[" not in texts

    # A caret sits inside the identifier being typed, so its quote is still
    # open. Tokenizing has to survive that or completion dies on the keystroke.
    @pytest.mark.parametrize("sql,sg_dialect", [
        ("SELECT * FROM `", "mysql"),
        ("SELECT `t1`.`", "mysql"),
        ("SELECT * FROM `ord", "mysql"),
        ("SELECT * FROM [", "sqlite"),
        ("SELECT * FROM [ord", "sqlite"),
        ('SELECT * FROM "', "postgres"),
        ('SELECT * FROM "ord', "postgres"),
    ])
    def test_unterminated_quote_still_tokenizes(self, sql, sg_dialect):
        tokens = tokenize(sql, sg_dialect)
        assert [t.text for t in tokens][:1] == ["SELECT"]

    def test_repair_does_not_shift_earlier_offsets(self):
        closed = tokenize("SELECT * FROM `t1`", "mysql")
        open_quote = tokenize("SELECT * FROM `t1", "mysql")
        assert [(t.text, t.start) for t in closed] == [(t.text, t.start) for t in open_quote]
