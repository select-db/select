package audit

import (
	"bytes"
	"testing"

	core "github.com/selectDb/dialect/core"
)

func ptr(s string) *string { return &s }

// Identical authz state must hash to the same value regardless of the order
// roles/permissions arrive in — that is what lets snapshots dedup.
func TestPrincipalHashIsOrderIndependent(t *testing.T) {
	a := Principal{
		Type:        PrincipalUser,
		ID:          "u1",
		WorkspaceID: "w1",
		Roles:       []Role{{ID: "r2", Name: "Writer"}, {ID: "r1", Name: "Reader"}},
		Permissions: []core.PermissionEntry{
			{TableName: ptr("orders"), Action: "select", Effect: "allow"},
			{TableName: ptr("customers"), Action: "select", Effect: "allow"},
		},
	}
	b := Principal{
		Type:        PrincipalUser,
		ID:          "u1",
		WorkspaceID: "w1",
		Roles:       []Role{{ID: "r1", Name: "Reader"}, {ID: "r2", Name: "Writer"}},
		Permissions: []core.PermissionEntry{
			{TableName: ptr("customers"), Action: "select", Effect: "allow"},
			{TableName: ptr("orders"), Action: "select", Effect: "allow"},
		},
	}
	if !bytes.Equal(a.Hash(), b.Hash()) {
		t.Fatalf("expected equal hashes for reordered identical state")
	}
}

// A different permission set must produce a different hash.
func TestPrincipalHashChangesWithPermissions(t *testing.T) {
	base := Principal{Type: PrincipalUser, ID: "u1", WorkspaceID: "w1", Roles: []Role{{ID: "r1", Name: "Reader"}}}
	changed := base
	changed.Permissions = []core.PermissionEntry{{TableName: ptr("orders"), Action: "select", Effect: "deny"}}

	if bytes.Equal(base.Hash(), changed.Hash()) {
		t.Fatalf("expected different hash when permissions differ")
	}
}
