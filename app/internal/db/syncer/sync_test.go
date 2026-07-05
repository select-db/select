package syncer

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncWith_ConfirmsCommits(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1", "Alice")
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedCommit(t, db, "commit-1", "u1", "ws1", "workspace")
	seedCommit(t, db, "commit-2", "u1", "ws1", "workspace")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Confirmed: []string{"commit-1", "commit-2"},
	}))

	err := s.syncWith(context.Background(), "u1", nil, nil, true, "ws1")
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM mutation_commit`).Scan(&count))
	assert.Equal(t, 0, count, "confirmed commits must be deleted")
}

func TestSyncWith_AppliesWorkspaceChange(t *testing.T) {
	db := newTestDB(t)

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Users: []map[string]any{
				{"id": "u1", "name": "Alice"},
			},
			Workspaces: []map[string]any{
				{"id": "ws1", "name": "Synced Workspace"},
			},
			WorkspaceToUser: []map[string]any{
				{"id": "wtu1", "workspace_id": "ws1", "user_id": "u1"},
			},
		},
	}))

	err := s.syncWith(context.Background(), "u1", nil, nil, true, "ws1")
	require.NoError(t, err)

	var name string
	require.NoError(t, db.QueryRow(`SELECT name FROM workspace WHERE id = 'ws1'`).Scan(&name))
	assert.Equal(t, "Synced Workspace", name)
}

func TestSyncWith_AppliesRoleChange(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Role: []map[string]any{
				{"id": "role-1", "workspace_id": "ws1", "name": "Admins"},
			},
		},
	}))

	err := s.syncWith(context.Background(), "u1", nil, nil, true, "ws1")
	require.NoError(t, err)

	var name string
	require.NoError(t, db.QueryRow(`SELECT name FROM role WHERE id = 'role-1'`).Scan(&name))
	assert.Equal(t, "Admins", name)
}

func TestSyncWith_AppliesRoleChange_UpdatesExisting(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedRole(t, db, "role-1", "ws1", "Old Name")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Role: []map[string]any{
				{"id": "role-1", "workspace_id": "ws1", "name": "New Name"},
			},
		},
	}))

	err := s.syncWith(context.Background(), "u1", nil, nil, true, "ws1")
	require.NoError(t, err)

	var name string
	require.NoError(t, db.QueryRow(`SELECT name FROM role WHERE id = 'role-1'`).Scan(&name))
	assert.Equal(t, "New Name", name)
}

func TestSyncWith_AppliesPermissionChange(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedRole(t, db, "role-1", "ws1", "Admins")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Permission: []map[string]any{
				{"id": "perm-1", "role_id": "role-1", "workspace_id": "ws1", "action": "select", "effect": "allow"},
			},
		},
	}))

	err := s.syncWith(context.Background(), "u1", nil, nil, true, "ws1")
	require.NoError(t, err)

	var effect string
	require.NoError(t, db.QueryRow(`SELECT effect FROM permission WHERE id = 'perm-1'`).Scan(&effect))
	assert.Equal(t, "allow", effect)
}

func TestSyncWith_DeletesRole(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedRole(t, db, "role-1", "ws1", "Admins")

	deletedAt := time.Now().UTC().Format(time.RFC3339)
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Role: []map[string]any{
				{"id": "role-1", "workspace_id": "ws1", "name": "Admins", "deleted_at": deletedAt},
			},
		},
	}))

	err := s.syncWith(context.Background(), "u1", nil, nil, true, "ws1")
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM role WHERE id = 'role-1'`).Scan(&count))
	assert.Equal(t, 0, count, "deleted role must be removed")
}

func TestSyncWith_DeletesPermission(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedRole(t, db, "role-1", "ws1", "Admins")
	_, err := db.Exec(`INSERT INTO permission (id, role_id, workspace_id, action, effect) VALUES ('perm-1', 'role-1', 'ws1', 'select', 'allow')`)
	require.NoError(t, err)

	deletedAt := time.Now().UTC().Format(time.RFC3339)
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Permission: []map[string]any{
				{"id": "perm-1", "role_id": "role-1", "workspace_id": "ws1", "action": "select", "effect": "allow", "deleted_at": deletedAt},
			},
		},
	}))

	err = s.syncWith(context.Background(), "u1", nil, nil, true, "ws1")
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM permission WHERE id = 'perm-1'`).Scan(&count))
	assert.Equal(t, 0, count, "deleted permission must be removed")
}

func TestSyncWith_EmitsRolesUpdated_OnRoleChange(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")

	called := false
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Role: []map[string]any{
				{"id": "role-1", "workspace_id": "ws1", "name": "Admins"},
			},
		},
	}))
	s.EmitRolesUpdated = func() { called = true }

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))
	assert.True(t, called, "EmitRolesUpdated must fire when roles in changes")
}

func TestSyncWith_EmitsRolesUpdated_OnPermissionChange(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedRole(t, db, "role-1", "ws1", "Admins")

	called := false
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Permission: []map[string]any{
				{"id": "perm-1", "role_id": "role-1", "workspace_id": "ws1", "action": "select", "effect": "allow"},
			},
		},
	}))
	s.EmitRolesUpdated = func() { called = true }

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))
	assert.True(t, called)
}

func TestSyncWith_NoEmitRolesUpdated_WhenNoRoleChanges(t *testing.T) {
	db := newTestDB(t)

	called := false
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Workspaces: []map[string]any{
				{"id": "ws1", "name": "Only Workspace"},
			},
		},
	}))
	s.EmitRolesUpdated = func() { called = true }

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, ""))
	assert.False(t, called, "EmitRolesUpdated must NOT fire for workspace-only changes")
}

func TestSyncWith_RestoresRole(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedRole(t, db, "role-1", "ws1", "Client Name")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Restored: []SyncRestoredItem{
			{
				ObjectID:  "role-1",
				TableName: "role",
				ServerPayload: map[string]any{
					"id": "role-1", "workspace_id": "ws1", "name": "Server Name",
				},
			},
		},
	}))

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))

	var name string
	require.NoError(t, db.QueryRow(`SELECT name FROM role WHERE id = 'role-1'`).Scan(&name))
	assert.Equal(t, "Server Name", name, "restored item must overwrite local state")
}

func TestSyncWith_EmitsRolesUpdated_OnRestoredRole(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedRole(t, db, "role-1", "ws1", "Local Name")

	called := false
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Restored: []SyncRestoredItem{
			{ObjectID: "role-1", TableName: "role", ServerPayload: map[string]any{
				"id": "role-1", "workspace_id": "ws1", "name": "Server Name",
			}},
		},
	}))
	s.EmitRolesUpdated = func() { called = true }

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))
	assert.True(t, called)
}

func TestSyncWith_UpdatesLastPulledAt(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	_, err := db.Exec(`INSERT INTO workspace_to_user (id, workspace_id, user_id, current) VALUES ('wtu1','ws1','u1',1)`)
	require.NoError(t, err)

	serverTime := time.Now().UTC().Truncate(time.Second)
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		ServerTime: serverTime,
	}))

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))

	var lastPulledAt sql.NullTime
	require.NoError(t, db.QueryRow(`SELECT last_pulled_at FROM workspace WHERE id = 'ws1'`).Scan(&lastPulledAt))
	require.True(t, lastPulledAt.Valid)
	assert.WithinDuration(t, serverTime, lastPulledAt.Time, time.Second)
}

func TestSyncWith_AppliesUserToRoleChange(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedUser(t, db, "u1", "Alice")
	seedRole(t, db, "role-1", "ws1", "Admins")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			UserToRole: []map[string]any{
				{"id": "utr-1", "user_id": "u1", "role_id": "role-1", "workspace_id": "ws1"},
			},
		},
	}))

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_to_role WHERE id = 'utr-1'`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestSyncWith_AppliesUserToRoleChange_UpdatesExisting(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedUser(t, db, "u1", "Alice")
	seedRole(t, db, "role-1", "ws1", "Admins")
	seedRole(t, db, "role-2", "ws1", "Members")
	_, err := db.Exec(`INSERT INTO user_to_role (id, user_id, role_id, workspace_id) VALUES ('utr-1', 'u1', 'role-1', 'ws1')`)
	require.NoError(t, err)

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			UserToRole: []map[string]any{
				{"id": "utr-1", "user_id": "u1", "role_id": "role-2", "workspace_id": "ws1"},
			},
		},
	}))

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))

	var roleID string
	require.NoError(t, db.QueryRow(`SELECT role_id FROM user_to_role WHERE id = 'utr-1'`).Scan(&roleID))
	assert.Equal(t, "role-2", roleID)
}

func TestSyncWith_DeletesUserToRole(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedUser(t, db, "u1", "Alice")
	seedRole(t, db, "role-1", "ws1", "Admins")
	_, err := db.Exec(`INSERT INTO user_to_role (id, user_id, role_id, workspace_id) VALUES ('utr-1', 'u1', 'role-1', 'ws1')`)
	require.NoError(t, err)

	deletedAt := time.Now().UTC().Format(time.RFC3339)
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			UserToRole: []map[string]any{
				{"id": "utr-1", "user_id": "u1", "role_id": "role-1", "workspace_id": "ws1", "deleted_at": deletedAt},
			},
		},
	}))

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_to_role WHERE id = 'utr-1'`).Scan(&count))
	assert.Equal(t, 0, count, "deleted user_to_role must be removed")
}

func TestSyncWith_EmitsRolesUpdated_OnUserToRoleChange(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedUser(t, db, "u1", "Alice")
	seedRole(t, db, "role-1", "ws1", "Admins")

	called := false
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			UserToRole: []map[string]any{
				{"id": "utr-1", "user_id": "u1", "role_id": "role-1", "workspace_id": "ws1"},
			},
		},
	}))
	s.EmitRolesUpdated = func() { called = true }

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))
	assert.True(t, called)
}

func TestSync_WithCurrentWorkspace(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1", "Alice")
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedWorkspaceToUser(t, db, "wtu1", "ws1", "u1", true)
	seedCommit(t, db, "commit-1", "u1", "ws1", "workspace")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Confirmed: []string{"commit-1"},
	}))

	require.NoError(t, s.Sync(context.Background(), "u1"))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM mutation_commit`).Scan(&count))
	assert.Equal(t, 0, count, "Sync must flush confirmed commits via current workspace path")
}

func TestSync_WithNoCurrentWorkspace(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1", "Alice")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Workspaces: []map[string]any{
				{"id": "ws1", "name": "Default Workspace"},
			},
		},
	}))

	require.NoError(t, s.Sync(context.Background(), "u1"))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM workspace`).Scan(&count))
	assert.Equal(t, 1, count, "Sync without current workspace must pull default workspace")
}

// Applies group, user_to_group and group_to_role in a SINGLE changes payload to
// prove applyChanges upserts them in FK-safe order (group before user_to_group /
// group_to_role, which reference it alongside user / role).
func TestSyncWith_AppliesGroupGraph_InOnePayload(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedUser(t, db, "u1", "Alice")
	seedRole(t, db, "role-1", "ws1", "Admins")

	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			Group: []map[string]any{
				{"id": "grp-1", "workspace_id": "ws1", "name": "Data Eng", "source": "local"},
			},
			UserToGroup: []map[string]any{
				{"id": "utg-1", "user_id": "u1", "group_id": "grp-1", "workspace_id": "ws1", "source": "local"},
			},
			GroupToRole: []map[string]any{
				{"id": "gtr-1", "group_id": "grp-1", "role_id": "role-1", "workspace_id": "ws1"},
			},
		},
	}))

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))

	var name string
	require.NoError(t, db.QueryRow(`SELECT name FROM "group" WHERE id = 'grp-1'`).Scan(&name))
	assert.Equal(t, "Data Eng", name)

	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_to_group WHERE id = 'utg-1'`).Scan(&n))
	assert.Equal(t, 1, n, "user_to_group must apply after its group")
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM group_to_role WHERE id = 'gtr-1'`).Scan(&n))
	assert.Equal(t, 1, n, "group_to_role must apply after its group and role")
}

func TestSyncWith_DeletesGroupToRoleAndUserToGroup(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedUser(t, db, "u1", "Alice")
	seedRole(t, db, "role-1", "ws1", "Admins")
	_, err := db.Exec(`INSERT INTO "group" (id, workspace_id, name) VALUES ('grp-1', 'ws1', 'Data Eng')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_to_group (id, user_id, group_id, workspace_id) VALUES ('utg-1', 'u1', 'grp-1', 'ws1')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO group_to_role (id, group_id, role_id, workspace_id) VALUES ('gtr-1', 'grp-1', 'role-1', 'ws1')`)
	require.NoError(t, err)

	deletedAt := time.Now().UTC().Format(time.RFC3339)
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			UserToGroup: []map[string]any{
				{"id": "utg-1", "user_id": "u1", "group_id": "grp-1", "workspace_id": "ws1", "deleted_at": deletedAt},
			},
			GroupToRole: []map[string]any{
				{"id": "gtr-1", "group_id": "grp-1", "role_id": "role-1", "workspace_id": "ws1", "deleted_at": deletedAt},
			},
		},
	}))

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))

	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_to_group WHERE id = 'utg-1'`).Scan(&n))
	assert.Equal(t, 0, n, "deleted user_to_group must be removed")
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM group_to_role WHERE id = 'gtr-1'`).Scan(&n))
	assert.Equal(t, 0, n, "deleted group_to_role must be removed")
}

func TestSyncWith_EmitsRolesUpdated_OnGroupToRoleChange(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws1", "My Workspace")
	seedRole(t, db, "role-1", "ws1", "Admins")
	_, err := db.Exec(`INSERT INTO "group" (id, workspace_id, name) VALUES ('grp-1', 'ws1', 'Data Eng')`)
	require.NoError(t, err)

	called := false
	s := newTestSyncer(t, db, fakeSyncHandler(SyncResponse{
		Changes: SyncChanges{
			GroupToRole: []map[string]any{
				{"id": "gtr-1", "group_id": "grp-1", "role_id": "role-1", "workspace_id": "ws1"},
			},
		},
	}))
	s.EmitRolesUpdated = func() { called = true }

	require.NoError(t, s.syncWith(context.Background(), "u1", nil, nil, true, "ws1"))
	assert.True(t, called, "group->role change must refresh effective roles in the UI")
}

func TestPayloadHasDeletedAt(t *testing.T) {
	assert.False(t, PayloadHasDeletedAt(map[string]any{}))
	assert.False(t, PayloadHasDeletedAt(map[string]any{"deleted_at": nil}))
	assert.False(t, PayloadHasDeletedAt(map[string]any{"deleted_at": ""}))
	assert.True(t, PayloadHasDeletedAt(map[string]any{"deleted_at": "2026-01-01T00:00:00Z"}))
	assert.True(t, PayloadHasDeletedAt(map[string]any{"deleted_at": float64(1)}))
}
