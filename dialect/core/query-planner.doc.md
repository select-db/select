# Query Planner

SELECT can show the **execution plan** for a query without running it. This reveals how the database intends to execute the SQL: which indexes it will use, how it will join tables, and the estimated cost of each operation.

## Supported dialects

- **PostgreSQL**: uses `EXPLAIN (FORMAT JSON)`
- **MySQL**: uses `EXPLAIN FORMAT=JSON`
- **SQLite**: uses `EXPLAIN QUERY PLAN`

## Reading the plan

The plan is displayed as a tree of operations. Each node shows:

- **Operation**: what the database is doing (e.g. Seq Scan, Index Scan, Hash Join, Sort)
- **Target table**: the table being accessed
- **Index**: the index used, if any
- **Condition**: the filter or join condition
- **Estimated cost**: relative cost of this operation
- **Estimated rows**: how many rows the database expects to process
- **% of total**: how much of the total cost this node represents

## What to look for

- **Seq Scan on large tables**: the database is reading every row. Consider adding an index.
- **High estimated rows with low actual rows** (in analyze mode): the statistics are stale. Run `ANALYZE` on the table.
- **Nested Loop joins on large datasets**: may indicate a missing index on the join column.

> The planner shows **estimates only**. For actual execution metrics, use [Query Analyze](/sql/query-analyze/).
