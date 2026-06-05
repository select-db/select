"""
Collect relations and column references from a parsed statement.
These are informational outputs included in every analyze() response.
"""
from __future__ import annotations

from sqlglot import exp
from sqlglot.optimizer.scope import ScopeType

from analysis.schema import pos


def collect_relations(scopes: list, default_schema: str) -> list[dict]:
    """Build the relation list from all scopes (physical + virtual tables)."""
    seen: set[tuple] = set()
    results = []

    scope_type_nesting = {
        ScopeType.ROOT:         0,
        ScopeType.CTE:          1,
        ScopeType.DERIVED_TABLE: 1,
        ScopeType.SUBQUERY:     1,
        ScopeType.UNION:        0,
        ScopeType.UDTF:         1,
    }

    for scope in scopes:
        nesting = scope_type_nesting.get(scope.scope_type, 0)

        for alias, source in scope.sources.items():
            if isinstance(source, exp.Table):
                table_name  = source.name
                schema_name = source.db or default_schema
                db_name     = source.catalog or ""
                effective_alias = alias if alias.lower() != table_name.lower() else ""
                line, col = pos(source)

                key = (schema_name.lower(), table_name.lower(), effective_alias.lower(), nesting)
                if key not in seen:
                    seen.add(key)
                    results.append({
                        "table":         table_name,
                        "schema":        schema_name,
                        "database":      db_name,
                        "alias":         effective_alias,
                        "is_virtual":    False,
                        "nesting_level": nesting,
                        "line":          line,
                        "col":           col,
                    })
            else:
                line, col = 1, 0
                key = ("", alias.lower(), alias.lower(), nesting)
                if key not in seen:
                    seen.add(key)
                    results.append({
                        "table":         alias,
                        "schema":        "",
                        "database":      "",
                        "alias":         alias,
                        "is_virtual":    True,
                        "nesting_level": nesting,
                        "line":          line,
                        "col":           col,
                    })

    return results


def collect_column_refs(stmt: exp.Expression) -> list[dict]:
    """Flat list of every exp.Column in the statement."""
    results = []
    for col in stmt.find_all(exp.Column):
        name = col.name
        if not name:
            continue
        line, col_pos = pos(col)
        results.append({
            "name":      name,
            "qualifier": col.table or "",
            "line":      line,
            "col":       col_pos,
        })
    return results
