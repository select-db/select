# Graph View

SELECT can visualize query results as charts. After running a query, switch to the graph view to plot your data.

![A query result plotted as a line chart, with the chart type, axes and series controls beside it.](/shots/graph.light.png)

## Chart types

Three chart types are available:

- **Bar** with optional stacking
- **Line** with data point markers
- **Area** with optional stacking

## Configuration

The graph panel lets you configure:

- **X column**: the column used for the horizontal axis
- **Y columns**: one or more columns for values (multiple columns create multi-series charts)
- **Series column**: pivot mode, where distinct values in this column become separate series
- **Stacked**: stack bar or area series on top of each other
- **Legend**: show or hide the series legend

## Pivot mode

When a **series column** is set, SELECT pivots the data automatically. Given a table with columns `date`, `country`, `revenue`, setting X to `date`, Y to `revenue`, and series to `country` produces one line per country.

## Example: revenue by product category

Combine pivot mode with runtime variables to build reusable report queries. This query uses `$PERIOD` and `$MIN_REVENUE` as runtime variables, so you can adjust the report without editing SQL:

```sql
SELECT
  date_trunc('week', o.created_at)::date AS week,
  p.category,
  SUM(o.amount) AS revenue
FROM orders o
JOIN products p ON p.id = o.product_id
WHERE o.created_at >= NOW() - $PERIOD::interval
GROUP BY week, p.category
HAVING SUM(o.amount) >= $MIN_REVENUE
ORDER BY week;
```

Set `$PERIOD` to `3 months` and `$MIN_REVENUE` to `1000`. Then configure the graph:

- **Chart type**: Area (stacked)
- **X column**: `week`
- **Y column**: `revenue`
- **Series column**: `category`

Each product category becomes its own stacked area. Change `$PERIOD` to `1 year` and re-run to zoom out, or swap the series column to `region` for a geographic breakdown. The same SQL file becomes multiple reports depending on the runtime values.

## Interactivity

- **Hover** any data point to see a tooltip with values across all series
- **Click** a series in the legend to hide or show it
- Markers (dots) are shown for datasets with 200 rows or fewer

## Limits

| Constraint              | Max    |
|-------------------------|--------|
| Bar chart categories    | 1,000  |
| Total rows              | 10,000 |
| Multi-Y series          | 8      |

When data exceeds these limits, a truncation notice is shown.
