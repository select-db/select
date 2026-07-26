package schema

import "testing"

func TestBuildDerivesRole(t *testing.T) {
	ents, errs := Build(RawSchema{Tables: []RawTable{RoleTable()}})
	if len(errs) != 0 {
		t.Fatalf("unexpected lint errors: %v", errs)
	}
	e := ents[0]
	if len(e.PrimaryKey) != 1 || e.PrimaryKey[0] != "id" {
		t.Fatalf("pk=%v", e.PrimaryKey)
	}
	// Only `name` is patchable; id/workspace_id/updated_at/deleted_at are system.
	var patch []string
	for _, f := range e.Fields {
		if f.Patchable {
			patch = append(patch, f.Name)
		}
	}
	if len(patch) != 1 || patch[0] != "name" {
		t.Fatalf("patchable=%v, want [name]", patch)
	}
	// api ops: list, get (open) + create, update, delete (require roles.manage)
	gated := map[string]bool{"create": true, "update": true, "delete": true}
	if len(e.API) != 5 {
		t.Fatalf("want 5 api ops, got %d: %+v", len(e.API), e.API)
	}
	for _, op := range e.API {
		if gated[op.Op] {
			if len(op.Requires) != 1 || op.Requires[0] != "roles.manage" {
				t.Fatalf("op %q should require roles.manage: %+v", op.Op, op)
			}
		} else if len(op.Requires) != 0 {
			t.Fatalf("op %q should be open: %+v", op.Op, op)
		}
	}
}

// buildRelations collapses repeated relation names into one Relation with a
// union of paths.
func TestBuildRelationsUnion(t *testing.T) {
	c := `@app.entity users
	  @app.relation groups: user.id -> user_to_group.user_id, user_to_group.group_id -> group.id
	  @app.relation permissions: user.id -> user_to_role.user_id, user_to_role.role_id -> role.id, role.name -> permission.role_name
	  @app.relation permissions: user.id -> user_to_group.user_id, user_to_group.group_id -> group_to_role.group_id, group_to_role.role_id -> role.id, role.name -> permission.role_name`
	rels := buildRelations(ParseTags(c).Relations)
	if len(rels) != 2 {
		t.Fatalf("want 2 relations, got %d", len(rels))
	}
	if rels[0].Name != "groups" || len(rels[0].Paths) != 1 || rels[0].Target != "group" {
		t.Fatalf("groups relation wrong: %+v", rels[0])
	}
	if rels[1].Name != "permissions" || len(rels[1].Paths) != 2 || rels[1].Target != "permission" {
		t.Fatalf("permissions relation wrong: %+v", rels[1])
	}
	last := rels[1].Paths[0][len(rels[1].Paths[0])-1]
	if last.ToTable != "permission" || last.ToCol != "role_name" {
		t.Fatalf("permissions path tail wrong: %+v", last)
	}
}

func TestLintFlagsMissingConvention(t *testing.T) {
	tbl := RoleTable()
	// Drop deleted_at -> a synced table now violates the convention.
	tbl.Columns = tbl.Columns[:len(tbl.Columns)-1]
	_, errs := Build(RawSchema{Tables: []RawTable{tbl}})
	if len(errs) == 0 {
		t.Fatal("expected a lint error for missing deleted_at")
	}
}
