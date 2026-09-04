# Export

SELECT can export query results to a file. The query runs in full (ignoring pagination), the output is formatted, and a save dialog lets you pick the destination.

## Supported formats

| Format             | Extension | Details                              |
|--------------------|-----------|--------------------------------------|
| **CSV (comma)**    | `.csv`    | Comma-separated values               |
| **CSV (semicolon)**| `.csv`    | Semicolon-separated (for locales using comma as decimal) |
| **JSON**           | `.json`   | Array of objects, pretty-printed     |

## How it works

1. The query executes against the associated database with all variables resolved
2. All rows are fetched (no streaming, no pagination)
3. The result is formatted in the chosen format
4. A native **Save** dialog opens with a suggested filename
5. The file is written to the selected path

> [!WARNING]
> Export fetches the **full result set** in memory. For very large results, the workspace `max_result_size_mb` limit (Settings → Workspace) still applies.

## Variable support

Runtime variables (`$VAR_NAME`) are resolved the same way as in normal query execution. If unresolved variables exist, the runtime variable form appears before export starts.
