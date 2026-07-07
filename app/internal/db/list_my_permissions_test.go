package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"

	"selectDb/internal/db/generated"

	"modernc.org/sqlite"
)

// The desktop app enforces query permissions locally via ListMyPermissions, so a
// role granted through a group (user_to_group -> group_to_role) must show up there,
// not just via directly-assigned roles. A hard-deleted group must drop its roles.
func TestListMyPermissions_IncludesGroupDerivedRoles(t *testing.T) {
	if !slices.Contains(sql.Drivers(), "sqlite3") {
		sql.Register("sqlite3", &sqlite.Driver{})
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := RunMigrationsAt(dbPath); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	q := generated.New(conn)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		if _, err := conn.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}

	// Alice belongs to a group; the group grants a role; the role allows SELECT.
	// She has NO direct user_to_role assignment.
	exec(`INSERT INTO user (id, name) VALUES ('u1','Alice')`)
	exec(`INSERT INTO workspace (id, name) VALUES ('ws1','WS')`)
	exec(`INSERT INTO role (id, workspace_id, name) VALUES ('r1','ws1','Analyst')`)
	exec(`INSERT INTO permission (id, role_id, workspace_id, action, effect) VALUES ('p1','r1','ws1','select','allow')`)
	exec(`INSERT INTO "group" (id, workspace_id, name) VALUES ('g1','ws1','Data Eng')`)
	exec(`INSERT INTO group_to_role (id, group_id, role_id, workspace_id) VALUES ('gtr1','g1','r1','ws1')`)
	exec(`INSERT INTO user_to_group (id, user_id, group_id, workspace_id) VALUES ('utg1','u1','g1','ws1')`)

	perms, err := q.ListMyPermissions(ctx, generated.ListMyPermissionsParams{UserID: "u1", WorkspaceID: "ws1"})
	if err != nil {
		t.Fatalf("ListMyPermissions: %v", err)
	}
	if len(perms) != 1 || perms[0].Action != "select" || perms[0].RoleID != "r1" {
		t.Fatalf("expected 1 group-derived permission (select via r1), got %+v", perms)
	}

	// The client hard-deletes a group on delete; its role must stop applying even
	// though the lingering user_to_group / group_to_role rows still reference it.
	exec(`DELETE FROM "group" WHERE id = 'g1'`)
	perms, err = q.ListMyPermissions(ctx, generated.ListMyPermissionsParams{UserID: "u1", WorkspaceID: "ws1"})
	if err != nil {
		t.Fatalf("ListMyPermissions after group delete: %v", err)
	}
	if len(perms) != 0 {
		t.Fatalf("expected 0 permissions after the group was deleted, got %+v", perms)
	}
}
