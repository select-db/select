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
	"github.com/selectDb/dialect/dialects"
)

// ExecuteLocal runs sql like StreamLocal and returns the whole result at once.
func ExecuteLocal(ctx context.Context, conn Conn, inst DBInstance, sql string, opts Options) *Result {
	result := &Result{}
	StreamLocal(ctx, conn, inst, sql, opts, resultSink{result})
	return result
}

// StreamLocal runs sql on conn and streams the result into sink: the columns,
// each row, then OnDone. Any failure ends the stream with one OnError.
func StreamLocal(ctx context.Context, conn Conn, inst DBInstance, sql string, opts Options, sink RowSink) {
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

	q, release, err := conn.acquire(ctx, sql)
	if err != nil {
		sink.OnError(err)
		return
	}
	defer release()

	start := time.Now()
	rows, err := q.QueryContext(ctx, sql, opts.Args...)
	durationMs := max1ms(time.Since(start).Milliseconds())
	if err != nil {
		// Retried as an exec, to report the rows it affected:
		//   - a write or DDL, which most drivers refuse to run as a query
		// Not retried:
		//   - a QueryRunsAll driver, whose query may already have run part of the script
		if _, runsAll := conn.DB.Driver().(QueryRunsAll); !runsAll {
			start := time.Now()
			if res, execErr := q.ExecContext(ctx, sql, opts.Args...); execErr == nil {
				affected, _ := res.RowsAffected()
				_ = sink.OnColumns(nil)
				if err := sink.OnDone(0, affected, max1ms(time.Since(start).Milliseconds())); err != nil {
					sink.OnError(err)
				}
				return
			}
		}
		// The query's error, the more useful one for a SELECT-shaped statement.
		sink.OnError(toQueryError(ctx, err))
		return
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		sink.OnError(fmt.Errorf("failed to get columns: %w", err))
		return
	}
	maskPositions, err := evaluateSeeForResult(conn, inst, inspected, columns)
	if err != nil {
		sink.OnError(err)
		return
	}
	if err := sink.OnColumns(columns); err != nil {
		sink.OnError(err)
		return
	}
	// After OnColumns, so the start event on the wire precedes this one.
	if s, ok := sink.(executedSink); ok {
		s.OnExecuted(durationMs)
	}
	colTypes, _ := rows.ColumnTypes()
	if s, ok := sink.(columnTypesSink); ok {
		s.SetColumnTypes(colTypes)
	}
	hasTZ := makeHasTZ(colTypes)

	maxBytes := effectiveMaxBytes(opts)
	var rowCount, bytesScanned int64
	for rows.Next() {
		if ctx.Err() != nil {
			sink.OnError(toQueryError(ctx, ctx.Err()))
			return
		}
		if opts.MaxRows > 0 && rowCount >= int64(opts.MaxRows) {
			if s, ok := sink.(truncatedSink); ok {
				s.OnTruncated()
			}
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
			sink.OnError(fmt.Errorf("result exceeded %dMB limit, refine your query or raise the max result size in the workspace settings", maxBytes/1024/1024))
			return
		}
		if err := sink.OnRow(values); err != nil {
			sink.OnError(err)
			return
		}
		rowCount++
	}
	if err := rows.Err(); err != nil {
		sink.OnError(toQueryError(ctx, err))
		return
	}
	if err := sink.OnDone(rowCount, 0, durationMs); err != nil {
		sink.OnError(err)
	}
}

// resultSink collects a stream into a Result. A failed stream leaves its error
// and no rows: a truncated result is worse than none.
type resultSink struct{ result *Result }

func (s resultSink) OnColumns(cols []string) error {
	s.result.Columns = cols
	return nil
}

func (s resultSink) OnRow(values []any) error {
	s.result.Rows = append(s.result.Rows, values)
	return nil
}

func (s resultSink) OnDone(rowCount, affected, durationMs int64) error {
	s.result.RowCount = int(rowCount)
	s.result.AffectedRows = affected
	s.result.DurationMs = durationMs
	return nil
}

func (s resultSink) OnError(err error) {
	s.result.Rows, s.result.RowCount = nil, 0
	s.result.Errors = []string{err.Error()}
	s.result.ErrorPosition = errorPosition(err)
}

func effectiveMaxBytes(opts Options) int64 {
	if opts.MaxBytes <= 0 || opts.MaxBytes > maxResultSizeHardCap {
		return maxResultSizeHardCap
	}
	return opts.MaxBytes
}

// checkPermissions parses sql and runs the action-level permission check
// (select/insert/update/delete, manage for the rest). The see check needs the driver's
// rows.Columns() output to map result positions, so it runs later from
// evaluateSeeForResult.
func checkPermissions(conn Conn, inst DBInstance, sql string) ([]core.InspectStatement, error) {
	if !conn.Perms.IsManaged(inst.ID) {
		return nil, nil
	}

	dialect := dialects.Get(inst.DBType)
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
	// The see check on the result columns needs the driver's columns and runs
	// later; what a statement tests rather than returns is known now, and a
	// write filtered on a hidden column reports its rows without returning any.
	if err := core.CheckSeePredicates(inspected, inst.ID, conn.Perms); err != nil {
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
	stmt, ok := FirstReturningStatement(inspected)
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

// queryError is a statement the database refused, with the 1-based character
// offset it reported, when it did.
type queryError struct {
	message  string
	position *int
}

func (e *queryError) Error() string {
	return e.message
}

// errorPosition returns where in the statement err occurred, or nil.
func errorPosition(err error) *int {
	var qe *queryError
	if errors.As(err, &qe) {
		return qe.position
	}
	return nil
}

// toQueryError words err for the user. ctx.Err() takes precedence: drivers
// often wrap it in their own type.
func toQueryError(ctx context.Context, err error) *queryError {
	if ctxErr := ctx.Err(); ctxErr != nil {
		err = ctxErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &queryError{message: "query timed out, raise the statement timeout in the workspace settings"}
	}
	if errors.Is(err, context.Canceled) {
		return &queryError{message: "query was cancelled"}
	}
	qe := &queryError{message: err.Error()}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Position != "" {
		if pos, parseErr := strconv.Atoi(pqErr.Position); parseErr == nil && pos > 0 {
			qe.position = &pos
		}
	}
	return qe
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
