package query

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// testFields mirrors a realistic exposed set (audit.event-shaped): enum text,
// free text, uuid, datetime, int, bool — with the OData operator sets the
// generator would advertise for each kind.
func testFields() *FieldSet {
	textOps := []Op{OpEq, OpNe, OpIn, OpNotIn, OpContains, OpStartsWith, OpEndsWith}
	uuidOps := []Op{OpEq, OpNe, OpIn, OpNotIn}
	timeOps := []Op{OpEq, OpNe, OpLt, OpLe, OpGt, OpGe, OpIn, OpNotIn}
	intOps := []Op{OpEq, OpNe, OpLt, OpLe, OpGt, OpGe, OpIn, OpNotIn}
	boolOps := []Op{OpEq, OpNe}
	return NewFieldSet([]Field{
		{Name: "id", Column: "id", Kind: KindUUID, Ops: uuidOps},
		{Name: "status", Column: "status", Kind: KindText, Ops: textOps, Enum: []string{"success", "error", "denied"}},
		{Name: "domain", Column: "domain", Kind: KindText, Ops: textOps, Enum: []string{"query", "iam", "datasource"}},
		{Name: "target_label", Column: "target_label", Kind: KindText, Ops: textOps},
		{Name: "occurred_at", Column: "occurred_at", Kind: KindTime, Ops: timeOps},
		{Name: "count", Column: "count", Kind: KindInt, Ops: intOps},
		{Name: "active", Column: "active", Kind: KindBool, Ops: boolOps},
	})
}

func TestCompileOK(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		want   string
		args   []any
	}{
		{"empty", "", "", nil},
		{"eq string", "status eq 'success'", "status = $2", []any{"success"}},
		{"ne string", "status ne 'error'", "status <> $2", []any{"error"}},
		{"and", "status eq 'success' and domain eq 'iam'", "(status = $2 AND domain = $3)", []any{"success", "iam"}},
		{"or", "status eq 'error' or status eq 'denied'", "(status = $2 OR status = $3)", []any{"error", "denied"}},
		{"not", "not status eq 'success'", "NOT status = $2", []any{"success"}},
		{"grouping", "(status eq 'error' or status eq 'denied') and domain eq 'iam'",
			"((status = $2 OR status = $3) AND domain = $4)", []any{"error", "denied", "iam"}},
		{"in", "domain in ('iam', 'query')", "domain IN ($2, $3)", []any{"iam", "query"}},
		{"not in", "domain not in ('query')", "domain NOT IN ($2)", []any{"query"}},
		{"time ge", "occurred_at ge 2026-01-01T00:00:00Z", "occurred_at >= $2",
			[]any{mustTime(t, "2026-01-01T00:00:00Z")}},
		{"int lt", "count lt 10", "count < $2", []any{int64(10)}},
		{"int negative", "count gt -5", "count > $2", []any{int64(-5)}},
		{"bool", "active eq true", "active = $2", []any{true}},
		{"contains", "contains(target_label, 'prod')", `target_label ILIKE $2 ESCAPE '\'`, []any{"%prod%"}},
		{"startswith", "startswith(target_label, 'prod')", `target_label ILIKE $2 ESCAPE '\'`, []any{"prod%"}},
		{"endswith", "endswith(target_label, 'db')", `target_label ILIKE $2 ESCAPE '\'`, []any{"%db"}},
		{"contains escapes", "contains(target_label, '50%_x')", `target_label ILIKE $2 ESCAPE '\'`, []any{`%50\%\_x%`}},
		{"is null", "target_label eq null", "target_label IS NULL", nil},
		{"is not null", "target_label ne null", "target_label IS NOT NULL", nil},
		{"uuid eq", "id eq '3f2504e0-4f89-41d3-9a0c-0305e82c3301'", "id = $2",
			[]any{"3f2504e0-4f89-41d3-9a0c-0305e82c3301"}},
		{"quote escape", "target_label eq 'O''Brien'", "target_label = $2", []any{"O'Brien"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, args, err := Compile(tc.filter, testFields(), 2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.want {
				t.Fatalf("sql:\n got %q\nwant %q", sql, tc.want)
			}
			if !argsEqual(args, tc.args) {
				t.Fatalf("args:\n got %#v\nwant %#v", args, tc.args)
			}
		})
	}
}

func TestCompileErrors(t *testing.T) {
	cases := []struct {
		name    string
		filter  string
		wantSub string // substring the message must contain
	}{
		{"unknown field", "stat eq 'x'", `unknown field "stat"`},
		{"did you mean", "staus eq 'success'", `did you mean "status"`},
		{"lists known fields", "nope eq 'x'", "Known fields: id, status, domain"},
		{"op not allowed", "status gt 'success'", `operator "gt" is not allowed on field "status"`},
		{"op not allowed lists allowed", "status gt 'success'", "allowed: eq, ne, in, not in"},
		{"type mismatch int", "count eq 'ten'", `field "count" expects an integer`},
		{"type mismatch string", "status eq 5", `field "status" expects a quoted string`},
		{"bad uuid", "id eq 'not-a-uuid'", `is not a valid uuid`},
		{"bad time", "occurred_at gt 'yesterday'", "expects an RFC 3339 date-time"},
		{"enum violation", "status eq 'pending'", `field "status" only accepts success, error, denied`},
		{"unbalanced parens", "(status eq 'success'", "unbalanced parentheses"},
		{"unterminated string", "status eq 'success", "unterminated string literal"},
		{"empty in list", "domain in ()", "expected a value"},
		{"trailing junk", "status eq 'success' bogus", "unexpected"},
		{"missing operator", "status", "expected an operator"},
		{"missing value", "status eq", "expected a value"},
		{"contains on non-text", "contains(count, 'x')", `operator "contains" is not allowed on field "count"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Compile(tc.filter, testFields(), 2)
			if err == nil {
				t.Fatalf("expected an error, got none")
			}
			var fe *InputError
			if !errors.As(err, &fe) {
				t.Fatalf("expected *InputError, got %T: %v", err, err)
			}
			if !strings.Contains(fe.Message, tc.wantSub) {
				t.Fatalf("message %q does not contain %q", fe.Message, tc.wantSub)
			}
		})
	}
}

// Params must start at the caller's offset (workspace id is $1).
func TestCompileParamOffset(t *testing.T) {
	sql, args, err := Compile("status eq 'success' and count gt 3", testFields(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if sql != "(status = $5 AND count > $6)" {
		t.Fatalf("got %q", sql)
	}
	if len(args) != 2 {
		t.Fatalf("args: %#v", args)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func argsEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if at, ok := a[i].(time.Time); ok {
			bt, ok := b[i].(time.Time)
			if !ok || !at.Equal(bt) {
				return false
			}
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
