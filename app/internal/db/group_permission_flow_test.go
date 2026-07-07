package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"

	"selectDb/internal/db"
	"selectDb/internal/db/generated"
	"selectDb/internal/group"
	"selectDb/internal/role"

	core "github.com/selectDb/dialect/core"
	"modernc.org/sqlite"
)

// End-to-end: drive the real Group service methods the desktop UI calls
// (CreateGroup -> AttachRoleToGroup -> AddUserToGroup), then confirm the granted
// role actually reaches the local permission check via GetMyPermissions and the
// dialect enforcement path. This is the flow the user exercised.
func TestGroupRoleFlow_AppliesToLocalPermissions(t *testing.T) {
	if !slices.Contains(sql.Drivers(), "sqlite3") {
		sql.Register("sqlite3", &sqlite.Driver{})
	}
	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := db.RunMigrationsAt(dbPath); err != nil {
		t.Fatalf("migrations: %v", err)
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

	// Current user + workspace (current=1 is what GetCurrentUser/Workspace key on).
	exec(`INSERT INTO user (id, name) VALUES ('u1','Alice')`)
	exec(`INSERT INTO workspace (id, name) VALUES ('ws1','WS')`)
	exec(`INSERT INTO workspace_to_user (id, workspace_id, user_id, current) VALUES ('wtu1','ws1','u1',1)`)
	// A role that denies SELECT on a specific datasource (db-scoped => enforceable).
	exec(`INSERT INTO role (id, workspace_id, name) VALUES ('r1','ws1','Restrictor')`)
	exec(`INSERT INTO permission (id, role_id, workspace_id, db_instance_id, action, effect)
	      VALUES ('p1','r1','ws1','db-1','select','deny')`)

	// Drive the actual service methods.
	gs := group.New(q)
	grp, err := gs.CreateGroup("Data Eng")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := gs.AttachRoleToGroup("r1", grp.ID); err != nil {
		t.Fatalf("AttachRoleToGroup: %v", err)
	}
	if err := gs.AddUserToGroup("u1", grp.ID); err != nil {
		t.Fatalf("AddUserToGroup: %v", err)
	}

	// The permission must now surface through the local resolver the app enforces on.
	perms, err := role.GetMyPermissions(q)
	if err != nil {
		t.Fatalf("GetMyPermissions: %v", err)
	}
	if len(perms) != 1 || perms[0].Action != "select" || perms[0].Effect != "deny" ||
		perms[0].DbInstanceID == nil || *perms[0].DbInstanceID != "db-1" {
		t.Fatalf("group-derived permission not resolved; got %+v", perms)
	}

	// And it must actually enforce: a SELECT on db-1 is denied.
	compiled := core.Compile(perms)
	if !compiled.IsManaged("db-1") {
		t.Fatalf("db-1 should be managed once a db-scoped rule exists")
	}
	stmts := []core.InspectStatement{{
		Operation: core.InspectOpSelect,
		Tables:    []core.InspectTable{{Schema: "public", Name: "t"}},
	}}
	if err := core.CheckQueryPermissions(stmts, "db-1", compiled); err == nil {
		t.Fatalf("expected SELECT on db-1 to be denied by the group-derived role")
	}
}
