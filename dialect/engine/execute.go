package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/selectDb/dialect/core"
)

func ExecuteLocal(ctx context.Context, conn Conn, inst DBInstance, sql string, opts Options) *Result {
	result := &Result{}

	inspected, err := checkPermissions(conn, inst, sql)
	if err != nil {
		result.Errors = []string{err.Error()}
		return result
	}

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	start := time.Now()
	rows, err := conn.DB.QueryContext(ctx, sql)
	result.DurationMs = max1ms(time.Since(start).Milliseconds())

	if err != nil {
		msg, pos := parseQueryError(ctx, err)
		result.Errors = []string{msg}
		result.ErrorPosition = pos

		// non-SELECT: try ExecContext
		start = time.Now()
		res, execErr := conn.DB.ExecContext(ctx, sql)
		result.DurationMs = max1ms(time.Since(start).Milliseconds())
		if execErr != nil {
			execMsg, execPos := parseQueryError(ctx, execErr)
			result.Errors = append(result.Errors, execMsg)
			if result.ErrorPosition == nil {
				result.ErrorPosition = execPos
			}
		} else if affected, err := res.RowsAffected(); err == nil {
			result.AffectedRows = affected
		}
		return result
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		result.Errors = []string{fmt.Sprintf("failed to get columns: %v", err)}
		return result
	}
	result.Columns = columns

	maskPositions, seeErr := evaluateSeeForResult(conn, inst, inspected, columns)
	if seeErr != nil {
		result.Errors = []string{seeErr.Error()}
		result.Columns = nil
		return result
	}

	colTypes, _ := rows.ColumnTypes()
	hasTZ := makeHasTZ(colTypes)

	maxBytes := effectiveMaxBytes(opts)
	var bytesScanned int64
	for rows.Next() {
		if ctx.Err() != nil {
			result.Errors = []string{ctx.Err().Error()}
			return result
		}
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to scan row: %v", err))
			continue
		}
		for i, v := range values {
			switch val := v.(type) {
			case []byte:
				values[i] = string(val)
			case time.Time:
				values[i] = formatTime(val, hasTZ, i)
			}
			bytesScanned += estimateValueBytes(values[i])
		}

		applyMask(values, maskPositions)

		if bytesScanned > maxBytes {
			result.Errors = []string{fmt.Sprintf("result exceeded %dMB limit, refine your query or raise max_result_size_mb in .config", maxBytes/1024/1024)}
			return result
		}
		result.Rows = append(result.Rows, values)
		result.RowCount++
	}

	if err := rows.Err(); err != nil {
		msg, pos := parseQueryError(ctx, err)
		result.Errors = []string{msg}
		result.ErrorPosition = pos
		// discard partial rows, a truncated result is worse than no result
		result.Rows = nil
		result.RowCount = 0
		return result
	}

	return result
}

func StreamLocal(
	ctx context.Context,
	conn Conn,
	inst DBInstance,
	sql string,
	opts Options,
	sink RowSink,
) {
	inspected, err := checkPermissions(conn, inst, sql)
	if err != nil {
		sink.OnError(err)
		return
	}

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	start := time.Now()
	rows, err := conn.DB.QueryContext(ctx, sql)
	durationMs := max1ms(time.Since(start).Milliseconds())

	if err != nil {
		// Mirror ExecuteLocal's fallback: a statement that doesn't return rows
		// (INSERT / UPDATE / DELETE / DDL) will fail QueryContext on most
		// drivers; retry via ExecContext to surface affected-row counts.
		execStart := time.Now()
		res, execErr := conn.DB.ExecContext(ctx, sql)
		execDurationMs := max1ms(time.Since(execStart).Milliseconds())
		if execErr == nil {
			_ = sink.OnColumns(nil)
			var affected int64
			if a, aerr := res.RowsAffected(); aerr == nil {
				affected = a
			}
			if doneErr := sink.OnDone(0, affected, execDurationMs); doneErr != nil {
				sink.OnError(doneErr)
			}
			return
		}
		// Surface the original SELECT-style error; it's the more useful
		// diagnostic for users writing SELECT-shaped statements.
		msg, _ := parseQueryError(ctx, err)
		sink.OnError(fmt.Errorf("%s", msg))
		return
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		sink.OnError(fmt.Errorf("failed to get columns: %w", err))
		return
	}

	maskPositions, seeErr := evaluateSeeForResult(conn, inst, inspected, columns)
	if seeErr != nil {
		sink.OnError(seeErr)
		return
	}

	if err := sink.OnColumns(columns); err != nil {
		sink.OnError(err)
		return
	}

	// Must fire after OnColumns so the start event on the wire precedes this
	if n, ok := sink.(interface{ OnExecuted(int64) }); ok {
		n.OnExecuted(durationMs)
	}

	colTypes, _ := rows.ColumnTypes()
	hasTZ := makeHasTZ(colTypes)
	setColumnTypes(sink, colTypes)

	maxBytes := effectiveMaxBytes(opts)
	maxRows := int64(opts.MaxRows) // 0 = unbounded
	var rowCount int64
	var bytesScanned int64
	truncated := false
	for rows.Next() {
		if ctx.Err() != nil {
			sink.OnError(ctx.Err())
			return
		}
		if maxRows > 0 && rowCount >= maxRows {
			truncated = true
			break
		}
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			sink.OnError(fmt.Errorf("failed to scan row: %w", err))
			return
		}
		for i, v := range values {
			switch val := v.(type) {
			case []byte:
				values[i] = string(val)
			case time.Time:
				values[i] = formatTime(val, hasTZ, i)
			}
			bytesScanned += estimateValueBytes(values[i])
		}

		applyMask(values, maskPositions)

		if bytesScanned > maxBytes {
			sink.OnError(fmt.Errorf("result exceeded %dMB limit, refine your query or raise max_result_size_mb in .config", maxBytes/1024/1024))
			return
		}
		if err := sink.OnRow(values); err != nil {
			sink.OnError(err)
			return
		}
		rowCount++
	}

	if err := rows.Err(); err != nil {
		msg, _ := parseQueryError(ctx, err)
		sink.OnError(fmt.Errorf("%s", msg))
		return
	}

	if truncated {
		if n, ok := sink.(interface{ OnTruncated() }); ok {
			n.OnTruncated()
		}
	}

	if err := sink.OnDone(rowCount, 0, durationMs); err != nil {
		sink.OnError(err)
	}
}

func effectiveMaxBytes(opts Options) int64 {
	if opts.MaxBytes <= 0 || opts.MaxBytes > maxResultSizeHardCap {
		return maxResultSizeHardCap
	}
	return opts.MaxBytes
}

// checkPermissions parses sql and runs the action-level permission check
// (select/insert/update/delete/ddl). The see check needs the driver's
// rows.Columns() output to map result positions, so it runs later from
// evaluateSeeForResult.
func checkPermissions(conn Conn, inst DBInstance, sql string) ([]core.InspectStatement, error) {
	if !conn.Perms.IsManaged(inst.ID) {
		return nil, nil
	}

	dialect := GetDialect(inst.DBType)
	if dialect == nil {
		return nil, fmt.Errorf("permission check failed: unsupported database type %q", inst.DBType)
	}

	if conn.Meta == nil {
		return nil, fmt.Errorf("permission check failed: metadata unavailable for %s", inst.ID)
	}

	inspected := Inspect(dialect, conn.Meta, sql)
	if err := core.CheckQueryPermissions(inspected, inst.ID, conn.Perms); err != nil {
		return inspected, err
	}
	return inspected, nil
}

// evaluateSeeForResult runs the see-permission check against the driver's
// actual result columns. Returns mask positions for the scan loop, or an
// error to be surfaced before any row is emitted.
func evaluateSeeForResult(conn Conn, inst DBInstance, inspected []core.InspectStatement, driverCols []string) ([]int, error) {
	if !conn.Perms.IsManaged(inst.ID) {
		return nil, nil
	}
	stmt, ok := FirstSelectStatement(inspected)
	if !ok {
		return nil, nil
	}
	return core.EvaluateSee(stmt, driverCols, inst.ID, conn.Perms)
}

func applyMask(values []any, maskPositions []int) {
	for _, i := range maskPositions {
		if i < 0 || i >= len(values) {
			continue
		}
		values[i] = core.MaskedValue
	}
}

// ctx.Err() takes precedence: drivers often wrap it in their own type
func parseQueryError(ctx context.Context, err error) (msg string, position *int) {
	if err == nil {
		return "", nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return "query timed out, raise statement_timeout_ms in .config file", nil
		}
		return "query was cancelled", nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "query timed out, raise statement_timeout_ms in .config file", nil
	}
	if errors.Is(err, context.Canceled) {
		return "query was cancelled", nil
	}
	msg = err.Error()
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Position != "" {
		if pos, parseErr := strconv.Atoi(pqErr.Position); parseErr == nil && pos > 0 {
			position = &pos
		}
	}
	return msg, position
}

func estimateValueBytes(v any) int64 {
	switch val := v.(type) {
	case string:
		return int64(len(val))
	case []byte:
		return int64(len(val))
	case nil:
		return 0
	default:
		// int, float64, bool, time.Time, etc, fixed overhead
		_ = val
		return 8
	}
}

func max1ms(ms int64) int64 {
	if ms < 1 {
		return 1
	}
	return ms
}

func setColumnTypes(sink RowSink, colTypes []*sql.ColumnType) {
	type setter interface {
		SetColumnTypes([]*sql.ColumnType)
	}
	if s, ok := sink.(setter); ok {
		s.SetColumnTypes(colTypes)
	}
}

func makeHasTZ(colTypes []*sql.ColumnType) []bool {
	if len(colTypes) == 0 {
		return nil
	}
	flags := make([]bool, len(colTypes))
	for i, ct := range colTypes {
		flags[i] = strings.Contains(strings.ToUpper(ct.DatabaseTypeName()), "TZ")
	}
	return flags
}

func formatTime(t time.Time, hasTZ []bool, col int) string {
	if len(hasTZ) > col && hasTZ[col] {
		return t.Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	return t.Format("2006-01-02T15:04:05.999999999")
}
