"""The group names this module emits are looked up in Go, so the two sides
have to hold the same set. A name on one side alone is silent: the caret
simply offers no word."""
import pathlib
import re

from completion import completion_context as cc

_GO = pathlib.Path(__file__).resolve().parents[3] / "completion.go"


def _emitted() -> set:
    """Every group name the caret classifier can answer with."""
    source = pathlib.Path(cc.__file__).read_text()
    names = set(re.findall(r'return "([a-z_]+)"', source))
    names |= set(re.findall(r'group = "([a-z_]+)"', source))
    names |= set(cc._CLAUSE_FOLLOWERS.values())
    names |= set(cc._WORD_FOLLOWERS.values())
    names |= set(cc._ALIASED.values())
    names |= {f"{statement.lower()}_{group}"
              for statement, group in cc._STATEMENT_GROUPS}
    names.add("statement")
    return names


def _declared() -> set:
    """Every group Go holds words for."""
    body = re.search(r"var keywordGroups = map\[string\]map\[string\]bool\{(.*?)\n\}\n",
                     _GO.read_text(), re.S).group(1)
    return set(re.findall(r'^\t"([a-z_]+)":', body, re.M))


def test_every_group_named_here_has_words_in_go():
    assert not _emitted() - _declared()


def test_every_group_with_words_is_named_here():
    assert not _declared() - _emitted()
