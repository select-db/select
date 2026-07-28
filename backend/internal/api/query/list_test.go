package query

import (
	"errors"
	"testing"
)

func listResource() (Resource, *FieldSet) {
	textOps := []Op{OpEq, OpNe, OpIn, OpNotIn, OpContains, OpStartsWith, OpEndsWith}
	uuidOps := []Op{OpEq, OpNe, OpIn, OpNotIn}
	timeOps := []Op{OpEq, OpNe, OpLt, OpLe, OpGt, OpGe, OpIn, OpNotIn}
	res := Resource{
		Table: "audit.event",
		PK:    "id",
		Fields: []Field{
			{Name: "id", Column: "id", Kind: KindUUID, Ops: uuidOps},
			{Name: "status", Column: "status", Kind: KindText, Ops: textOps, Enum: []string{"success", "error", "denied"}},
			{Name: "updated_at", Column: "updated_at", Kind: KindTime, Ops: timeOps},
		},
	}
	return res, NewFieldSet(res.Fields)
}

func TestCursorRoundTrip(t *testing.T) {
	for _, c := range []Cursor{
		{Value: "2026-01-01T00:00:00Z", ID: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
		{Value: "a|b|c", ID: "id-1"}, // value with separators survives
		{Value: "", ID: "id-2"},
	} {
		got, err := DecodeCursor(EncodeCursor(c))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got != c {
			t.Fatalf("round trip: got %+v want %+v", got, c)
		}
	}
	if _, err := DecodeCursor("!!!not base64!!!"); err == nil {
		t.Fatal("expected error for garbage cursor")
	}
}

func TestParseSort(t *testing.T) {
	_, fs := listResource()
	cases := []struct {
		in       string
		wantName string
		wantDesc bool
	}{
		{"", "updated_at", true},
		{"-updated_at", "updated_at", true},
		{"status", "status", false},
		{"+status", "status", false},
		{"-status", "status", true},
	}
	for _, tc := range cases {
		f, desc, err := parseSort(tc.in, fs)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if f.Name != tc.wantName || desc != tc.wantDesc {
			t.Fatalf("%q: got (%s, desc=%v) want (%s, desc=%v)", tc.in, f.Name, desc, tc.wantName, tc.wantDesc)
		}
	}
	for _, bad := range []struct{ in, sub string }{
		{"nope", `unknown field "nope"`},
		{"staus", `did you mean "status"`},
		{"a,b", "single field"},
	} {
		_, _, err := parseSort(bad.in, fs)
		var ie *InputError
		if !errors.As(err, &ie) || !contains(ie.Message, bad.sub) {
			t.Fatalf("%q: got %v, want InputError containing %q", bad.in, err, bad.sub)
		}
	}
}

func TestBuildListSQL(t *testing.T) {
	res, fs := listResource()
	updatedAt, _ := fs.lookup("updated_at")
	id, _ := fs.lookup("id")

	t.Run("basic", func(t *testing.T) {
		sql, args, err := buildListSQL(res, fs, "ws1", "", updatedAt, true, nil, 50)
		mustSQL(t, err, sql,
			"SELECT id, status, updated_at FROM audit.event WHERE workspace_id = $1 AND deleted_at IS NULL ORDER BY updated_at DESC, id DESC LIMIT $2")
		wantArgs(t, args, []any{"ws1", 51})
	})

	t.Run("with filter", func(t *testing.T) {
		sql, args, err := buildListSQL(res, fs, "ws1", "status eq 'success'", updatedAt, true, nil, 50)
		mustSQL(t, err, sql,
			"SELECT id, status, updated_at FROM audit.event WHERE workspace_id = $1 AND deleted_at IS NULL AND status = $2 ORDER BY updated_at DESC, id DESC LIMIT $3")
		wantArgs(t, args, []any{"ws1", "success", 51})
	})

	t.Run("with cursor on updated_at", func(t *testing.T) {
		cur := &Cursor{Value: "2026-01-01T00:00:00Z", ID: "abc"}
		sql, args, err := buildListSQL(res, fs, "ws1", "", updatedAt, true, cur, 50)
		mustSQL(t, err, sql,
			"SELECT id, status, updated_at FROM audit.event WHERE workspace_id = $1 AND deleted_at IS NULL AND (updated_at, id) < ($2::timestamptz, $3::uuid) ORDER BY updated_at DESC, id DESC LIMIT $4")
		wantArgs(t, args, []any{"ws1", mustTime(t, "2026-01-01T00:00:00Z"), "abc", 51})
	})

	t.Run("sort by pk uses single-column keyset", func(t *testing.T) {
		cur := &Cursor{Value: "abc", ID: "abc"}
		sql, args, err := buildListSQL(res, fs, "ws1", "", id, false, cur, 50)
		mustSQL(t, err, sql,
			"SELECT id, status, updated_at FROM audit.event WHERE workspace_id = $1 AND deleted_at IS NULL AND id > $2::uuid ORDER BY id ASC LIMIT $3")
		wantArgs(t, args, []any{"ws1", "abc", 51})
	})

	t.Run("filter and cursor param numbering", func(t *testing.T) {
		cur := &Cursor{Value: "2026-01-01T00:00:00Z", ID: "abc"}
		sql, _, err := buildListSQL(res, fs, "ws1", "status eq 'error'", updatedAt, true, cur, 25)
		mustSQL(t, err, sql,
			"SELECT id, status, updated_at FROM audit.event WHERE workspace_id = $1 AND deleted_at IS NULL AND status = $2 AND (updated_at, id) < ($3::timestamptz, $4::uuid) ORDER BY updated_at DESC, id DESC LIMIT $5")
	})
}

func mustSQL(t *testing.T, err error, got, want string) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("sql:\n got %s\nwant %s", got, want)
	}
}

func wantArgs(t *testing.T, got, want []any) {
	t.Helper()
	if !argsEqual(got, want) {
		t.Fatalf("args:\n got %#v\nwant %#v", got, want)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
