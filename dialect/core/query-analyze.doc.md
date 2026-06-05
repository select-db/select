# Query Analyze

Query analyze **executes the query** and measures actual performance. Unlike the planner which only shows estimates, analyze reports real execution times, row counts, and cache behavior.

## How it works

SELECT runs the query with `EXPLAIN ANALYZE` (and `BUFFERS` on PostgreSQL). The query executes fully, but results are discarded. You get the execution plan enriched with actual metrics.

> Because the query runs for real, **write statements will modify data**. Use with caution on INSERT, UPDATE, and DELETE.

## What you see

On top of everything the [Query Planner](/sql/query-planner/) shows, analyze adds:

- **Actual rows**: how many rows each operation actually processed (vs estimated)
- **Actual time**: real execution time per operation in milliseconds
- **Exclusive time**: time spent in this operation only, excluding children
- **Peak memory**: memory usage for sort and hash operations

### Cache statistics (PostgreSQL)

On PostgreSQL, analyze also reports buffer cache behavior:

- **Shared hit/read blocks**: how many data blocks came from cache vs disk
- **Temp read/write blocks**: temporary storage used for sorts and hashes
- **Local hit/read blocks**: blocks from temporary tables

High shared hit ratios mean your data fits in memory. High read counts mean the database is going to disk.

## Warnings

SELECT automatically flags anomalies:

- **Actual rows much higher than estimated**: statistics may be stale
- **High cost operations**: nodes consuming a disproportionate share of total cost
