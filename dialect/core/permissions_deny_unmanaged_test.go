package core

import (
	"errors"
	"testing"
)

// WithDenyUnmanaged flips the default-allow rule on so that a statement
// against a DB the role has no entries on gets denied instead of silently
// allowed. Used by the MCP server (and any future autonomous client).
//
// Mirrors the structure of the existing permission test cases.
func TestWithDenyUnmanaged_FlipsDefault(t *testing.T) {
	cases := []struct {
		name    string
		entries []PermissionEntry
		stmt    InspectStatement
		dbID    string
		wantErr bool
	}{
		{
			name:    "no entries on this DB: default-allow allows",
			entries: nil,
			stmt:    selectRes("public", "t1", "c1"),
			dbID:    testDBID,
			wantErr: false,
		},
		{
			name:    "no entries on this DB: default-deny denies",
			entries: nil,
			stmt:    selectRes("public", "t1", "c1"),
			dbID:    testDBID,
			wantErr: true,
		},
		{
			name:    "entry on other DB: default-deny denies this DB",
			entries: []PermissionEntry{pe("other-db", "public", "t1", "c1", "select", "allow")},
			stmt:    selectRes("public", "t1", "c1"),
			dbID:    testDBID,
			wantErr: true,
		},
		{
			name:    "explicit allow on this DB: default-deny respects allow",
			entries: []PermissionEntry{pe(testDBID, "public", "t1", "c1", "select", "allow")},
			stmt:    selectRes("public", "t1", "c1"),
			dbID:    testDBID,
			wantErr: false,
		},
		{
			name:    "wildcard allow on this DB: default-deny respects allow",
			entries: []PermissionEntry{pe(testDBID, "public", "", "", "select", "allow")},
			stmt:    selectRes("public", "t1", "c1"),
			dbID:    testDBID,
			wantErr: false,
		},
		{
			name:    "no entries elsewhere either: default-deny still denies this DB",
			entries: []PermissionEntry{pe(testDBID, "private", "t9", "c9", "select", "allow")},
			stmt:    selectRes("public", "t1", "c1"),
			dbID:    testDBID,
			wantErr: true,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compiled := Compile(tc.entries)
			// The first case keeps default-allow; everything else flips to deny.
			if i > 0 {
				compiled = compiled.WithDenyUnmanaged()
			}
			err := CheckQueryPermissions([]InspectStatement{tc.stmt}, tc.dbID, compiled)
			if tc.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				var pde *PermissionDeniedError
				if errors.As(err, &pde) {
					t.Fatalf("want no error, got %v", pde)
				}
				t.Fatalf("want no error, got %v", err)
			}
		})
	}
}

func TestWithDenyUnmanaged_DoesNotMutateOriginal(t *testing.T) {
	base := Compile(nil)
	if base.IsManaged(testDBID) {
		t.Fatalf("base should not consider %s managed", testDBID)
	}
	strict := base.WithDenyUnmanaged()
	if !strict.IsManaged(testDBID) {
		t.Fatalf("strict should consider every DB managed")
	}
	// Original copy must keep default-allow semantics.
	if base.IsManaged(testDBID) {
		t.Fatalf("WithDenyUnmanaged mutated the original")
	}
}
