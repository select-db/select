package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ngrok/sqlmw"

	"selectDb/internal/db/syncer"
)

type SqlInterceptor struct {
	Db *InternalDb
	sqlmw.NullInterceptor
}

// extract query payload, operation type and table
func (idb *InternalDb) parseQuery(query string, args []driver.NamedValue) (string, []driver.NamedValue, string) {
	operation, mappedArgs, table, err := mapArgsAndOperation(query, args)
	if err != nil {
		return "", args, ""
	}
	return operation, mappedArgs, table
}

// commitMutation save the mutation locally and push it to the
// graph to update fontend and to the backend
func (idb *InternalDb) commitMutation(
	query string,
	operation string,
	payload []driver.NamedValue,
	table string,
) error {
	if query == "" || operation == "" || len(payload) == 0 || table == "" {
		return nil
	}

	err := idb.Syncer.CommitMutation(&syncer.QueueMutationParams{
		Query:     query,
		TableName: table,
		Operation: operation,
		Args:      payload,
	})

	if err != nil {
		return err
	}

	return nil
}

// execArgs returns a copy of args with Name fields cleared for the driver.
func execArgs(args []driver.NamedValue) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	copy(out, args)
	for i := range out {
		out[i].Name = ""
	}
	return out
}

// ConnExecContext intercepts non-query executions (INSERT/UPDATE/DELETE)
func (s *SqlInterceptor) ConnExecContext(
	ctx context.Context,
	conn driver.ExecerContext,
	query string,
	args []driver.NamedValue,
) (driver.Result, error) {
	operation, payload, table := s.Db.parseQuery(query, args)
	execResult, err := conn.ExecContext(ctx, query, execArgs(payload))
	if err == nil {
		// @todo handle commitMutation errors
		err = s.Db.commitMutation(query, operation, payload, table)
	}

	return execResult, err
}

// ConnQueryContext intercepts queries (SELECT) or mutating queries that return rows
func (s *SqlInterceptor) ConnQueryContext(
	ctx context.Context,
	conn driver.QueryerContext,
	query string,
	args []driver.NamedValue,
) (context.Context, driver.Rows, error) {
	operation, payload, table := s.Db.parseQuery(query, args)
	queryRows, queryErr := conn.QueryContext(ctx, query, execArgs(payload))
	if queryErr == nil && operation != "" && table != "" {
		// Defer the commit until rows are closed so the INSERT cursor is finalized
		// before SaveCommit opens a new write, otherwise SQLite returns SQLITE_BUSY.
		db := s.Db
		return ctx, &commitOnCloseRows{
			Rows: queryRows,
			commit: func() {
				_ = db.commitMutation(query, operation, payload, table)
			},
		}, nil
	}
	return ctx, queryRows, queryErr
}

// commitOnCloseRows wraps driver.Rows and triggers the commit when the rows are closed,
// ensuring the INSERT cursor is finalized before SaveCommit opens a new write.
type commitOnCloseRows struct {
	driver.Rows
	commit func()
	done   bool
}

func (r *commitOnCloseRows) Close() error {
	err := r.Rows.Close()
	if !r.done {
		r.done = true
		r.commit()
	}
	return err
}

func mapArgsAndOperation(query string, args []driver.NamedValue) (string, []driver.NamedValue, string, error) {
	q := strings.ToUpper(query)
	switch {
	case strings.Contains(q, "INSERT"):
		mapped, table, ok := tryInsertMapping(query, args)
		if ok {
			return "insert", mapped, table, nil
		}
	case strings.Contains(q, "UPDATE"):
		mapped, table, ok := tryUpdateMapping(query, args)
		if ok {
			return "update", mapped, table, nil
		}
	case strings.Contains(q, "DELETE"):
		mapped, table, ok := tryDeleteMapping(query, args)
		if ok {
			return "delete", mapped, table, nil
		}
	}
	return "none", nil, "", errors.New("could not map query")
}

var (
	fieldOrdinalRegexp = regexp.MustCompile(`(\w+)\s*=\s*\?(\d+)`)
	updateTableRegexp  = regexp.MustCompile(`(?is).*?\bupdate\s+([^\s]+)\s+set`)
)

func tryUpdateMapping(query string, args []driver.NamedValue) ([]driver.NamedValue, string, bool) {
	table := ""
	if m := updateTableRegexp.FindStringSubmatch(query); len(m) > 1 {
		table = m[1]
	}

	matches := fieldOrdinalRegexp.FindAllStringSubmatch(query, -1)
	if len(matches) == 0 {
		return args, table, false
	}

	ordinalToField := make(map[int]string)
	for _, m := range matches {
		field := m[1]
		ordinal, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		ordinalToField[ordinal] = field
	}

	for i := range args {
		if field, ok := ordinalToField[args[i].Ordinal]; ok {
			args[i].Name = field
		} else {
			args[i].Name = fmt.Sprintf("unknown_field_%d", args[i].Ordinal)
		}
	}
	return args, table, true
}

var (
	firstParensRegexp = regexp.MustCompile(`\(([^)]*)\)`)
	insertTableRegexp = regexp.MustCompile(`(?is).*?\binsert\s+into\s+([^\s(]+)`)
)

func tryInsertMapping(query string, args []driver.NamedValue) ([]driver.NamedValue, string, bool) {
	table := ""
	if m := insertTableRegexp.FindStringSubmatch(query); len(m) > 1 {
		table = m[1]
	}

	match := firstParensRegexp.FindStringSubmatch(query)
	if len(match) < 2 {
		return nil, table, false
	}

	columns := splitAndTrim(match[1])
	for i := range args {
		if args[i].Ordinal <= 0 || args[i].Ordinal > len(columns) {
			args[i].Name = fmt.Sprintf("arg_%d", args[i].Ordinal)
		} else {
			args[i].Name = columns[args[i].Ordinal-1]
		}
	}
	return args, table, true
}

var (
	deleteTableRegexp = regexp.MustCompile(`(?is).*?\bdelete\s+from\s+([^\s]+)`)
)

func tryDeleteMapping(query string, args []driver.NamedValue) ([]driver.NamedValue, string, bool) {
	table := ""
	if m := deleteTableRegexp.FindStringSubmatch(query); len(m) > 1 {
		table = m[1]
	}

	matches := fieldOrdinalRegexp.FindAllStringSubmatch(query, -1)
	if len(matches) == 0 {
		return args, table, false
	}

	ordinalToField := make(map[int]string)
	for _, m := range matches {
		field := m[1]
		ordinal, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		ordinalToField[ordinal] = field
	}

	for i := range args {
		if field, ok := ordinalToField[args[i].Ordinal]; ok {
			args[i].Name = field
		} else {
			args[i].Name = fmt.Sprintf("unknown_field_%d", args[i].Ordinal)
		}
	}
	return args, table, true
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
