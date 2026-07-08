package audit

import (
	"reflect"
	"testing"

	"backend/db/db_types"
	"backend/db/generated"
)

func TestToSnake(t *testing.T) {
	cases := map[string]string{
		"DbInstanceID": "db_instance_id",
		"RoleID":       "role_id",
		"WorkspaceID":  "workspace_id",
		"Action":       "action",
		"Effect":       "effect",
	}
	for in, want := range cases {
		if got := toSnake(in); got != want {
			t.Errorf("toSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

// Diff turns sqlc rows/params into a clean before/after map, generically. No
// per-column code, nullable wrappers come out as values (not {Value,Valid}).
func TestDiffGeneric(t *testing.T) {
	after := generated.UpsertPermissionParams{
		Action: db_types.NewJSONNullString("select"),
		Effect: db_types.NewJSONNullString("allow"),
	}

	// creation: only "after"
	created := Diff(nil, after)
	if _, ok := created["before"]; ok {
		t.Fatalf("creation diff must not include before")
	}
	a, ok := created["after"].(map[string]any)
	if !ok {
		t.Fatalf("after not a map: %T", created["after"])
	}
	if a["action"] != "select" || a["effect"] != "allow" {
		t.Fatalf("snake_case values wrong: %+v", a)
	}

	// update: a nil *Row is treated as no-before; a real before is included
	var nilRow *generated.AppPermission
	if _, ok := Diff(nilRow, after)["before"]; ok {
		t.Fatalf("nil pointer before must be omitted")
	}
	updated := Diff(generated.AppPermission{Action: db_types.NewJSONNullString("select")}, after)
	if _, ok := updated["before"].(map[string]any); !ok {
		t.Fatalf("update diff must include before, got %v", reflect.TypeOf(updated["before"]))
	}
}
