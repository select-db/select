package core_references

import "testing"

func TestRelationRefForQualifier_alias(t *testing.T) {
	norm := func(s string) string { return s }
	refs := []RelationRef{
		{Schema: "public", Table: "ApiKey", Alias: "ca", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
	}
	r := RelationRefForQualifier(refs, "ca", 10, norm)
	if r == nil || r.Table != "ApiKey" {
		t.Fatalf("got %+v", r)
	}
}

func TestRelationRefForSingleIdent_prefersAlias(t *testing.T) {
	norm := func(s string) string { return s }
	refs := []RelationRef{
		{Schema: "public", Table: "ApiKey", Alias: "ca", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
	}
	r := RelationRefForSingleIdent(refs, "ca", 5, norm)
	if r == nil || r.Table != "ApiKey" {
		t.Fatalf("got %+v", r)
	}
}

func TestVirtualTableForQualifier_CTENotActiveBeforeScope(t *testing.T) {
	norm := func(s string) string { return s }
	vtabs := []RelationRef{
		{Table: "c", ScopeStartPos: 50, ScopeEndPos: 200, NestingLevel: 0},
	}
	if v := VirtualTableForQualifier(vtabs, "c", 30, norm); v != nil {
		t.Fatalf("caret before CTE scope should not match, got %+v", v)
	}
	if v := VirtualTableForQualifier(vtabs, "c", 100, norm); v == nil || v.Table != "c" {
		t.Fatalf("got %+v", v)
	}
}
