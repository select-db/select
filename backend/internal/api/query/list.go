package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Convention columns, mirroring apigen's schema tenancy/soft-delete/cursor
// columns. Kept as local constants so this runtime doesn't depend on the
// generator package.
const (
	tenantColumn     = "workspace_id"
	softDeleteColumn = "deleted_at"
	defaultSort      = "-updated_at"
	defaultLimit     = 50
	maxLimit         = 200
)

// Querier is the read surface List needs, satisfied by *sql.DB (and *sql.Tx).
// Taking it as a parameter keeps this package free of a DB-package dependency
// and unit-testable with a fake.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Resource is the table a list runs over: its schema-qualified name, the name
// of its primary-key field (the keyset tiebreaker), and its exposed fields in
// response order. All of these come from the generator's IR, never from the
// request, so table/column identifiers are trusted.
type Resource struct {
	Table  string
	PK     string
	Fields []Field
}

// Request is a parsed list query.
type Request struct {
	WorkspaceID string
	Filter      string
	Sort        string
	Limit       int
	Cursor      string
}

// Page is the {data, next_cursor} envelope. NextCursor is empty on the last
// page. Data is always a non-nil slice so it serializes as [] not null.
type Page struct {
	Data       []map[string]any `json:"data"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

// List runs a workspace-scoped, soft-delete-filtered read with the request's
// OData $filter, sort, and keyset cursor, returning one page plus the cursor for
// the next. An InputError (bad $filter/sort/cursor) is the caller's 400; any
// other error is a 500.
func List(ctx context.Context, q Querier, res Resource, req Request) (Page, error) {
	fs := NewFieldSet(res.Fields)

	sortField, desc, err := parseSort(req.Sort, fs)
	if err != nil {
		return Page{}, err
	}
	limit := clampLimit(req.Limit)

	var cur *Cursor
	if req.Cursor != "" {
		c, err := DecodeCursor(req.Cursor)
		if err != nil {
			return Page{}, err
		}
		cur = &c
	}

	sqlText, args, err := buildListSQL(res, fs, req.WorkspaceID, req.Filter, sortField, desc, cur, limit)
	if err != nil {
		return Page{}, err
	}

	rows, err := q.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return Page{}, err
	}
	data, err := scanRows(rows, res.Fields)
	if err != nil {
		return Page{}, err
	}

	page := Page{Data: data}
	if len(data) > limit { // the extra row proves there's another page
		page.Data = data[:limit]
		last := page.Data[limit-1]
		page.NextCursor = EncodeCursor(Cursor{
			Value: cursorValueString(sortField, last[sortField.Name]),
			ID:    fmt.Sprint(last[res.PK]),
		})
	}
	if page.Data == nil {
		page.Data = []map[string]any{}
	}
	return page, nil
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultLimit
	}
	if n > maxLimit {
		return maxLimit
	}
	return n
}

// parseSort reads a "[-]field" sort expression (leading '-' = descending),
// defaulting to newest-first. The field must be exposed and non-json.
func parseSort(s string, fs *FieldSet) (Field, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		s = defaultSort
	}
	if strings.ContainsRune(s, ',') {
		return Field{}, false, inputErr("sort accepts a single field, e.g. -updated_at or name")
	}
	desc, name := false, s
	switch {
	case strings.HasPrefix(s, "-"):
		desc, name = true, s[1:]
	case strings.HasPrefix(s, "+"):
		name = s[1:]
	}
	name = strings.TrimSpace(name)
	f, ok := fs.lookup(name)
	if !ok {
		if sug := fs.suggest(name); sug != "" {
			return Field{}, false, inputErr("cannot sort by unknown field %q; did you mean %q? Known fields: %s", name, sug, fs.known())
		}
		return Field{}, false, inputErr("cannot sort by unknown field %q. Known fields: %s", name, fs.known())
	}
	if f.Kind == KindJSON {
		return Field{}, false, inputErr("cannot sort by field %q (a json value has no order)", name)
	}
	return f, desc, nil
}

// buildListSQL assembles the parameterized read. Every value is a bound arg;
// only table/column identifiers (from the generator's IR) are interpolated.
func buildListSQL(res Resource, fs *FieldSet, workspaceID, filter string, sortField Field, desc bool, cur *Cursor, limit int) (string, []any, error) {
	pk, ok := fs.lookup(res.PK)
	if !ok {
		return "", nil, fmt.Errorf("query: resource %s has no primary-key field %q", res.Table, res.PK)
	}

	cols := make([]string, len(res.Fields))
	for i, f := range res.Fields {
		cols[i] = f.Column
	}

	args := []any{workspaceID}
	where := []string{tenantColumn + " = $1", softDeleteColumn + " IS NULL"}
	param := 2

	if strings.TrimSpace(filter) != "" {
		frag, fargs, err := Compile(filter, fs, param)
		if err != nil {
			return "", nil, err
		}
		where = append(where, frag)
		args = append(args, fargs...)
		param += len(fargs)
	}

	if cur != nil {
		// Keyset over (sort, pk). The pk (a random uuid) only has to be a unique,
		// stable tiebreaker with a total order — which uuid is — so pagination has
		// no skips or dupes even though ids aren't monotonic; it is in fact
		// required because the sort column (e.g. updated_at) is not unique. The
		// bound values carry explicit ::type casts so the comparison never leans
		// on Postgres inferring a uuid from a string param inside a row compare.
		op := "<"
		if !desc {
			op = ">"
		}
		if sortField.Column == pk.Column {
			where = append(where, fmt.Sprintf("%s %s $%d::%s", pk.Column, op, param, sqlType(pk.Kind)))
			args = append(args, cur.ID)
			param++
		} else {
			v, err := cursorValueArg(sortField, cur.Value)
			if err != nil {
				return "", nil, err
			}
			where = append(where, fmt.Sprintf("(%s, %s) %s ($%d::%s, $%d::%s)",
				sortField.Column, pk.Column, op, param, sqlType(sortField.Kind), param+1, sqlType(pk.Kind)))
			args = append(args, v, cur.ID)
			param += 2
		}
	}

	dir := "DESC"
	if !desc {
		dir = "ASC"
	}
	order := sortField.Column + " " + dir
	if sortField.Column != pk.Column {
		order += ", " + pk.Column + " " + dir
	}

	sqlText := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT $%d",
		strings.Join(cols, ", "), res.Table, strings.Join(where, " AND "), order, param)
	args = append(args, limit+1)
	return sqlText, args, nil
}

// scanRows reads the result set into one map per row, keyed by API field name,
// with each value converted to a JSON-friendly Go type by the field's kind.
func scanRows(rows *sql.Rows, fields []Field) ([]map[string]any, error) {
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		dests := make([]any, len(fields))
		for i, f := range fields {
			dests[i] = scanDest(f.Kind)
		}
		if err := rows.Scan(dests...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(fields))
		for i, f := range fields {
			m[f.Name] = jsonValue(dests[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// sqlType is the Postgres type a kind's bound value is cast to in the keyset
// comparison, so string-bound values (uuid, inet) compare as the column type
// rather than as text.
func sqlType(k Kind) string {
	switch k {
	case KindInt:
		return "bigint"
	case KindTime:
		return "timestamptz"
	case KindBool:
		return "boolean"
	case KindUUID:
		return "uuid"
	case KindInet:
		return "inet"
	default: // text
		return "text"
	}
}

func scanDest(k Kind) any {
	switch k {
	case KindInt:
		return new(sql.NullInt64)
	case KindBool:
		return new(sql.NullBool)
	case KindTime:
		return new(sql.NullTime)
	case KindJSON:
		return new([]byte)
	default: // text, uuid, inet
		return new(sql.NullString)
	}
}

func jsonValue(dest any) any {
	switch d := dest.(type) {
	case *sql.NullInt64:
		if d.Valid {
			return d.Int64
		}
	case *sql.NullBool:
		if d.Valid {
			return d.Bool
		}
	case *sql.NullTime:
		if d.Valid {
			return d.Time.UTC().Format(time.RFC3339Nano)
		}
	case *[]byte:
		if *d != nil {
			return json.RawMessage(*d)
		}
	case *sql.NullString:
		if d.Valid {
			return d.String
		}
	}
	return nil
}

// cursorValueString serializes a row's sort-key value for the cursor; the
// inverse of cursorValueArg.
func cursorValueString(f Field, v any) string {
	if v == nil {
		return ""
	}
	switch f.Kind {
	case KindInt:
		if n, ok := v.(int64); ok {
			return strconv.FormatInt(n, 10)
		}
	case KindBool:
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b)
		}
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// cursorValueArg types a cursor's stored sort value for the keyset comparison,
// matching the sort column's type. A value that no longer parses is a stale or
// tampered cursor (400).
func cursorValueArg(f Field, s string) (any, error) {
	switch f.Kind {
	case KindInt:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, inputErr("invalid cursor")
		}
		return n, nil
	case KindTime:
		ts, err := parseTime(s)
		if err != nil {
			return nil, inputErr("invalid cursor")
		}
		return ts, nil
	case KindBool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, inputErr("invalid cursor")
		}
		return b, nil
	default: // text, uuid, inet
		return s, nil
	}
}
