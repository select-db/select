# Inline Editing

Click any cell in the results table to edit its value directly. SELECT builds UPDATE statements from your changes so you can review them before they hit the database.

## Requirements

Inline editing works when the query result includes all primary key columns for the target table. If primary keys are missing, SELECT shows a notification listing the missing columns. Primary key columns themselves are not editable.

Computed or expression columns (e.g. `COUNT(*)`, `a + b`) cannot be edited.

## Reviewing and applying

The table header shows the number of edited rows. Two actions are available:

- **Review** generates UPDATE statements grouped by row. Each modified row produces one UPDATE with a WHERE clause built from its primary key values. The SQL opens as a new temporary file so you can inspect it, adjust it, or run it like any other query.
- **Rollback** discards all pending edits and resets the table to its original state.

You can also rollback individual cells by clicking the rollback icon that appears on hover.

## Type handling

Values are formatted in the generated SQL according to their column type:

| Input        | SQL output              |
| ------------ | ----------------------- |
| empty + NULL | `NULL`                  |
| boolean      | `TRUE` / `FALSE`        |
| numeric      | `42`, `3.14` (unquoted) |
| text         | `'escaped value'`       |

Column names that collide with reserved keywords are automatically quoted for the target dialect.
