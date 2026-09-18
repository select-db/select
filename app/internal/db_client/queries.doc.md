# Query Execution

Run a query with `Cmd+Enter`. SELECT executes the SQL against one or more associated databases and displays results in a table below the editor.

## Streaming (proxified connections)

When using **proxified connections**, results are streamed progressively. You see rows as they arrive instead of waiting for the full result set. The table fills incrementally while the query is still running.

For **local connections**, results are fetched in a single round-trip.

## Limits

Two **workspace** settings (configured in **Settings → Workspace**, shared with
your team) control execution bounds:

| Setting                  | Default | Max   | Description                            |
|--------------------------|---------|-------|----------------------------------------|
| **statement_timeout_ms** | 30000   | none  | Max query execution time (milliseconds)|
| **max_result_size_mb**   | 100     | 250   | Max result size before truncation (MB) |

These limits apply to both local and proxified queries. When a timeout fires, SELECT cancels the query on the database side. If the result exceeds the size limit, it is truncated and a notice is shown.

Set `statement_timeout_ms` to `0` to disable the timeout entirely. This is useful for long-running migrations or bulk operations.

## Proxified connections: limitations

Proxified queries are executed on the remote backend and streamed back over HTTP. This introduces constraints that do not apply to local connections:

- **45-minute HTTP ceiling.** Regardless of `statement_timeout_ms`, the HTTP transport enforces a hard 45-minute timeout. Queries that exceed this are terminated. Setting `statement_timeout_ms` to `0` disables the query-level timeout but does not remove the HTTP ceiling.
- **No resume on disconnect.** If the network connection drops mid-query (Wi-Fi change, NAT timeout, laptop sleep), the stream is lost. The query may keep running on the database, but SELECT has no way to reconnect and retrieve the results.
- **Connection pool pressure.** Each running query holds one connection from the backend's pool for its entire duration. Long-running queries reduce the number of connections available to other users on the same datasource.

### Long-running statements

For operations that take more than a few minutes (migrations, bulk inserts, index rebuilds), connect to the database directly using its native client (`psql`, `mysql`, etc.) instead of running them through a proxified connection. Direct connections are not subject to the HTTP ceiling and survive transient network issues.

If you must run a long statement through SELECT, keep `statement_timeout_ms` high enough to cover the expected duration and ensure your network connection is stable for the entire time. There is no progress indicator for DDL statements that return no rows.

## Result caching

Results are cached on your machine. Switching tabs and coming back shows the cached result instantly without re-executing. Running the query again replaces the cache.
