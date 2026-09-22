"""What the reference collector reports for the clauses that name a relation
somewhere other than a FROM."""
import server

SCHEMA = {
    "schemas": [
        {
            "name": "main",
            "tables": [
                {"name": "t1", "columns": [{"name": "c1", "type": "INTEGER"}]},
                {"name": "t2", "columns": [{"name": "c1", "type": "INTEGER"}]},
            ],
        },
    ],
}


def _relations(sql: str, sg_dialect: str = "postgresql") -> list[str]:
    response = server._dispatch({
        "action": "collect_references",
        "sql": sql,
        "dialect": sg_dialect,
        "schema": SCHEMA,
        "default_schema": "main",
    })
    return [relation["table"] for relation in response["relations"]]


class TestDeleteUsing:
    def test_it_reads_the_relation(self):
        assert _relations("DELETE FROM t1 USING t2 WHERE t1.c1 = t2.c1") == ["t1", "t2"]

    def test_it_reads_several(self):
        assert _relations("DELETE FROM t1 USING t2, (SELECT 1) s WHERE 1 = 1") == [
            "t1", "t2", "s"]

    def test_every_dialect(self):
        for sg_dialect in ("postgresql", "mysql", "sqlite"):
            assert _relations("DELETE FROM t1 USING t2 WHERE 1 = 1", sg_dialect) == [
                "t1", "t2"]


class TestUpdateFrom:
    def test_it_reads_the_relation(self):
        assert _relations("UPDATE t1 SET c1 = 1 FROM t2 WHERE t1.c1 = t2.c1") == [
            "t1", "t2"]
