package syncer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/api/syncer/types"
)

func TestSync_EmptyRequest(t *testing.T) {
	newTestDB(t)

	resp, _, err := Sync(context.Background(), newID(), []string{}, nil, nil, &types.SyncRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Confirmed)
	assert.Empty(t, resp.Restored)
	assert.WithinDuration(t, time.Now(), resp.ServerTime, 5*time.Second)
}

func TestAuthorize_OwnerCanRenameWorkspace(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "Old Name", ownerID)

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "UPDATE",
			TableName:   "workspace",
			ObjectID:    wsID,
			WorkspaceID: wsID,
			UserID:      ownerID,
			CreatedAt:   time.Now().Add(time.Second),
			Payload:     map[string]any{"name": "New Name"},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)

	var name string
	require.NoError(t, conn.QueryRow(`SELECT name FROM app.workspace WHERE id = $1::uuid`, wsID).Scan(&name))
	assert.Equal(t, "New Name", name)
}

func TestAuthorize_OwnerCanDeleteWorkspace(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "My Workspace", ownerID)

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "delete",
			TableName:   "workspace",
			ObjectID:    wsID,
			WorkspaceID: wsID,
			UserID:      ownerID,
			Payload:     map[string]any{"id": wsID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)

	var deletedAt *time.Time
	require.NoError(t, conn.QueryRow(`SELECT deleted_at FROM app.workspace WHERE id = $1::uuid`, wsID).Scan(&deletedAt))
	assert.NotNil(t, deletedAt, "workspace must be soft-deleted")
}

func TestAuthorize_NonOwnerCannotDeleteWorkspace(t *testing.T) {
	conn := newTestDB(t)
	ownerID, memberID, wsID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "My Workspace", ownerID)

	// member of wsID, not its owner (ownedWorkspaceIDs nil)
	resp, _, err := Sync(context.Background(), memberID, []string{wsID}, nil, nil, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "delete",
			TableName:   "workspace",
			ObjectID:    wsID,
			WorkspaceID: wsID,
			UserID:      memberID,
			Payload:     map[string]any{"id": wsID},
		}},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Confirmed, "non-owner workspace delete must not be confirmed")

	var deletedAt *time.Time
	require.NoError(t, conn.QueryRow(`SELECT deleted_at FROM app.workspace WHERE id = $1::uuid`, wsID).Scan(&deletedAt))
	assert.Nil(t, deletedAt, "workspace must NOT be soft-deleted by a non-owner")
}

// roleCommit builds a minimal role INSERT commit.
func roleCommit(userID, wsID, roleID string) types.Commit {
	return types.Commit{
		ID:          newID(),
		Operation:   "INSERT",
		TableName:   "role",
		ObjectID:    roleID,
		WorkspaceID: wsID,
		UserID:      userID,
		Payload:     map[string]any{"id": roleID, "workspace_id": wsID, "name": "Test Role"},
	}
}

func TestAuthorize_OwnerCanCreateRole(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{roleCommit(ownerID, wsID, roleID)},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)

	idUUID, err := db_types.NewJSONNullUUIDFromString(roleID)
	require.NoError(t, err)
	_, err = db.Queries.GetRoleByID(context.Background(), idUUID)
	assert.NoError(t, err, "role must exist after owner commit")
}

func TestAuthorize_OwnerCanUpdateRole(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Old Name")

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "UPDATE",
			TableName:   "role",
			ObjectID:    roleID,
			WorkspaceID: wsID,
			UserID:      ownerID,
			CreatedAt:   time.Now().Add(time.Second),
			Payload:     map[string]any{"id": roleID, "workspace_id": wsID, "name": "New Name"},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)

	idUUID, err := db_types.NewJSONNullUUIDFromString(roleID)
	require.NoError(t, err)
	role, err := db.Queries.GetRoleByID(context.Background(), idUUID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", role.Name.String)
}

func TestAuthorize_OwnerCanDeleteRole(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "To Delete")

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "delete",
			TableName:   "role",
			ObjectID:    roleID,
			WorkspaceID: wsID,
			UserID:      ownerID,
			Payload:     map[string]any{"id": roleID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)

	idUUID, err := db_types.NewJSONNullUUIDFromString(roleID)
	require.NoError(t, err)
	role, err := db.Queries.GetRoleByID(context.Background(), idUUID)
	require.NoError(t, err)
	assert.True(t, role.DeletedAt.Valid, "role must be soft-deleted after owner delete")
}

func TestAuthorize_NonOwnerCannotCreateRole(t *testing.T) {
	conn := newTestDB(t)
	ownerID, memberID, wsID, roleID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)

	resp, _, err := Sync(context.Background(), memberID, []string{wsID}, nil, nil, &types.SyncRequest{
		PendingCommits: []types.Commit{roleCommit(memberID, wsID, roleID)},
	})
	require.NoError(t, err)
	// row doesn't exist on server → client gets the commit confirmed so it drops the pending commit
	require.Len(t, resp.Confirmed, 1)
	assert.Empty(t, resp.Restored)

	var count int
	require.NoError(t, conn.QueryRow(`SELECT count(*) FROM app.role WHERE id = $1::uuid`, roleID).Scan(&count))
	assert.Equal(t, 0, count, "role must not be created")
}

func TestAuthorize_NonOwnerCannotUpdateRole(t *testing.T) {
	conn := newTestDB(t)
	ownerID, memberID, wsID, roleID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Original Name")

	resp, _, err := Sync(context.Background(), memberID, []string{wsID}, nil, nil, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "UPDATE",
			TableName:   "role",
			ObjectID:    roleID,
			WorkspaceID: wsID,
			UserID:      memberID,
			CreatedAt:   time.Now().Add(time.Second),
			Payload:     map[string]any{"id": roleID, "workspace_id": wsID, "name": "Hijacked Name"},
		}},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Confirmed)
	require.Len(t, resp.Restored, 1)

	item := resp.Restored[0]
	assert.Equal(t, "role", item.TableName)
	assert.Equal(t, roleID, item.ObjectID)
	payload, ok := item.ServerPayload.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Original Name", payload["name"])

	var name string
	require.NoError(t, conn.QueryRow(`SELECT name FROM app.role WHERE id = $1::uuid`, roleID).Scan(&name))
	assert.Equal(t, "Original Name", name)
}

func TestAuthorize_NonOwnerCannotUpdatePermission(t *testing.T) {
	conn := newTestDB(t)
	ownerID, memberID, wsID, roleID, permID := newID(), newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedPermission(t, conn, permID, roleID, wsID, "select", "allow")

	resp, _, err := Sync(context.Background(), memberID, []string{wsID}, nil, nil, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "UPDATE",
			TableName:   "permission",
			ObjectID:    permID,
			WorkspaceID: wsID,
			UserID:      memberID,
			CreatedAt:   time.Now().Add(time.Second),
			Payload: map[string]any{
				"id":           permID,
				"role_id":      roleID,
				"workspace_id": wsID,
				"action":       "select",
				"effect":       "deny",
			},
		}},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Confirmed)
	require.Len(t, resp.Restored, 1)

	item := resp.Restored[0]
	assert.Equal(t, "permission", item.TableName)
	assert.Equal(t, permID, item.ObjectID)
	payload, ok := item.ServerPayload.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "select", payload["action"])
	assert.Equal(t, "allow", payload["effect"])

	var effect string
	require.NoError(t, conn.QueryRow(`SELECT effect FROM app.permission WHERE id = $1::uuid`, permID).Scan(&effect))
	assert.Equal(t, "allow", effect)
}

func TestAuthorize_MismatchedUserIDRejected(t *testing.T) {
	conn := newTestDB(t)
	ownerID, otherID, wsID, roleID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)

	commit := roleCommit(ownerID, wsID, roleID)
	commit.UserID = otherID // commit claims to be from otherID but caller is ownerID

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{commit},
	})
	require.NoError(t, err)
	// row doesn't exist → confirmed so client drops the commit; nothing applied
	require.Len(t, resp.Confirmed, 1)
	assert.Empty(t, resp.Restored, "commit with mismatched UserID must be rejected")
}

func TestAuthorize_WorkspaceNotInUserListRejected(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, otherWsID, roleID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{roleCommit(ownerID, otherWsID, roleID)},
	})
	require.NoError(t, err)
	// row doesn't exist → confirmed so client drops the commit; nothing applied
	require.Len(t, resp.Confirmed, 1)
	assert.Empty(t, resp.Restored, "commit for workspace not in user's list must be rejected")
}

func TestAuthorize_PayloadForeignWorkspaceRejected(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, foreignWsID, roleID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)

	commit := roleCommit(ownerID, wsID, roleID)
	// commit targets wsID (allowed) but payload references a foreign workspace
	commit.Payload = map[string]any{"id": roleID, "workspace_id": foreignWsID, "name": "Smuggled"}

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{commit},
	})
	require.NoError(t, err)
	// row doesn't exist → confirmed so client drops the commit; nothing applied
	require.Len(t, resp.Confirmed, 1)
	assert.Empty(t, resp.Restored, "payload referencing foreign workspace must be rejected")
}

// TestAuthorize_PayloadWorkspaceMismatchRejected guards against a commit where
// c.WorkspaceID = W1 but payload.workspace_id = W2 (both valid workspaces for
// the caller). Without the c.WorkspaceID == payload.workspace_id check, an owner
// of W2 could write into W1 by relying on the "owner of any workspace" fast-path.
func TestAuthorize_PayloadWorkspaceMismatchRejected(t *testing.T) {
	conn := newTestDB(t)
	ownerID, ws1ID, ws2ID, roleID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, ws1ID, "WS1", ownerID)
	seedWorkspace(t, conn, ws2ID, "WS2", ownerID)

	commit := roleCommit(ownerID, ws1ID, roleID)
	// payload claims workspace_id = ws2ID while commit targets ws1ID
	commit.Payload = map[string]any{"id": roleID, "workspace_id": ws2ID, "name": "Smuggled"}

	resp, _, err := Sync(context.Background(), ownerID, []string{ws1ID, ws2ID}, nil, []string{ws1ID, ws2ID}, &types.SyncRequest{
		PendingCommits: []types.Commit{commit},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)
	assert.Empty(t, resp.Restored, "payload workspace_id mismatch must be rejected even when caller owns both workspaces")

	var count int
	require.NoError(t, conn.QueryRow(`SELECT count(*) FROM app.role WHERE id = $1::uuid`, roleID).Scan(&count))
	assert.Equal(t, 0, count, "role must not be created when payload workspace_id mismatches commit workspace")
}
