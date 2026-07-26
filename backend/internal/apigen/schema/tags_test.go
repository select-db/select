package schema

import (
	"reflect"
	"testing"
)

// --- table-level directives (ParseTags) ---

func TestParseTags_TableDirectives(t *testing.T) {
	tg := ParseTags("@app.entity users @app.sync @app.scope workspace")
	if tg.Entity != "users" {
		t.Errorf("entity = %q, want users", tg.Entity)
	}
	if !tg.Sync {
		t.Error("sync flag not set")
	}
	if tg.Scope != "workspace" {
		t.Errorf("scope = %q, want workspace", tg.Scope)
	}
}

func TestParseTags_NoDirectives(t *testing.T) {
	for _, in := range []string{"", "just a prose comment, no tags", "mentions app but not @app.x"} {
		tg := ParseTags(in)
		if tg.Entity != "" || tg.Sync || tg.Scope != "" || len(tg.API) != 0 || len(tg.Relations) != 0 {
			t.Errorf("ParseTags(%q) should be empty, got %+v", in, tg)
		}
	}
}

func TestParseTags_UnknownDirectiveIgnored(t *testing.T) {
	tg := ParseTags("prose @app.sync @app.bogus whatever @app.entity foo")
	if !tg.Sync || tg.Entity != "foo" {
		t.Errorf("unknown directive should be skipped, got %+v", tg)
	}
}

// An op-group (a|b|c) shares one requires clause; ungrouped ops stay open, order
// is preserved.
func TestParseTags_APIGroupSharesRequires(t *testing.T) {
	tg := ParseTags("@app.api.list|get @app.api.create|update|delete requires roles.manage")
	want := []APIOp{
		{Op: "list"}, {Op: "get"},
		{Op: "create", Requires: []string{"roles.manage"}},
		{Op: "update", Requires: []string{"roles.manage"}},
		{Op: "delete", Requires: []string{"roles.manage"}},
	}
	if !reflect.DeepEqual(tg.API, want) {
		t.Fatalf("api ops = %+v\nwant %+v", tg.API, want)
	}
}

func TestParseTags_APIMultiActionRequires(t *testing.T) {
	tg := ParseTags("@app.api.update requires roles.manage, users.manage @app.api.delete")
	want := []APIOp{
		{Op: "update", Requires: []string{"roles.manage", "users.manage"}},
		{Op: "delete"},
	}
	if !reflect.DeepEqual(tg.API, want) {
		t.Fatalf("api ops = %+v\nwant %+v", tg.API, want)
	}
}

func TestParseTags_Relation(t *testing.T) {
	tg := ParseTags("@app.relation groups: user.id -> user_to_group.user_id, user_to_group.group_id -> group.id")
	if len(tg.Relations) != 1 {
		t.Fatalf("want 1 relation decl, got %+v", tg.Relations)
	}
	r := tg.Relations[0]
	if r.Name != "groups" || len(r.Path) != 2 {
		t.Fatalf("relation parsed wrong: %+v", r)
	}
	if r.Path[0] != (Edge{FromTable: "user", FromCol: "id", ToTable: "user_to_group", ToCol: "user_id"}) {
		t.Errorf("edge[0] = %+v", r.Path[0])
	}
	if r.Path[1] != (Edge{FromTable: "user_to_group", FromCol: "group_id", ToTable: "group", ToCol: "id"}) {
		t.Errorf("edge[1] = %+v", r.Path[1])
	}
}

// A repeated relation name yields one decl per line (buildRelations unions them);
// a malformed line is skipped rather than panicking.
func TestParseTags_RelationRepeatsAndMalformed(t *testing.T) {
	tg := ParseTags("@app.relation r: a.id -> b.a_id @app.relation r: a.id -> c.a_id @app.relation broken no colon")
	if len(tg.Relations) != 2 {
		t.Fatalf("want 2 relation decls (broken one skipped), got %+v", tg.Relations)
	}
}

// --- column-level directives (ParseColumnTags) ---

func TestParseColumnTags_All(t *testing.T) {
	ct := ParseColumnTags(`Outcome. @app.hide @app.immutable @app.values [success, error, denied] ` +
		`@app.ops [eq, in] @app.lookup users @app.json.paths [$.a, $.b]`)
	if !ct.Hide || !ct.Immutable {
		t.Errorf("hide=%v immutable=%v, want both true", ct.Hide, ct.Immutable)
	}
	if !reflect.DeepEqual(ct.Values, []string{"success", "error", "denied"}) {
		t.Errorf("values = %v", ct.Values)
	}
	if !reflect.DeepEqual(ct.Ops, []string{"eq", "in"}) {
		t.Errorf("ops = %v", ct.Ops)
	}
	if ct.Lookup != "users" {
		t.Errorf("lookup = %q", ct.Lookup)
	}
	if !reflect.DeepEqual(ct.JSONPaths, []string{"$.a", "$.b"}) {
		t.Errorf("json.paths = %v", ct.JSONPaths)
	}
}

func TestParseColumnTags_None(t *testing.T) {
	ct := ParseColumnTags("just a description, no tags")
	if ct.Hide || ct.Immutable || len(ct.Values) != 0 || len(ct.Ops) != 0 || ct.Lookup != "" || len(ct.JSONPaths) != 0 {
		t.Errorf("expected empty column tags, got %+v", ct)
	}
}
