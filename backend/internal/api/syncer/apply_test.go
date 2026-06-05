package syncer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/api/syncer/types"
)

// updateRoleCommit builds a role UPDATE commit with an explicit CreatedAt.
func updateRoleCommit(userID, wsID, roleID, name string, createdAt time.Time) types.Commit {
	return types.Commit{
		ID:          newID(),
		Operation:   "UPDATE",
		TableName:   "role",
		ObjectID:    roleID,
		WorkspaceID: wsID,
		UserID:      userID,
		CreatedAt:   createdAt,
		Payload:     map[string]any{"id": roleID, "workspace_id": wsID, "name": name},
	}
}

func TestApply_ClientWins_NewerCommit(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Old Name")

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{
			updateRoleCommit(ownerID, wsID, roleID, "New Name", time.Now().Add(time.Second)),
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)
	assert.Empty(t, resp.Restored)

	var name string
	require.NoError(t, conn.QueryRow(`SELECT name FROM app.role WHERE id = $1::uuid`, roleID).Scan(&name))
	assert.Equal(t, "New Name", name)
}

func TestApply_ServerWins_OlderCommit(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Server Name")

	// zero CreatedAt is before any real updated_at → server wins
	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{
			updateRoleCommit(ownerID, wsID, roleID, "Client Name", time.Time{}),
		},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Confirmed)
	require.Len(t, resp.Restored, 1)

	item := resp.Restored[0]
	assert.Equal(t, "role", item.TableName)
	assert.Equal(t, roleID, item.ObjectID)

	payload, ok := item.ServerPayload.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Server Name", payload["name"])
	assert.Equal(t, roleID, payload["id"])

	// row unchanged on server
	var name string
	require.NoError(t, conn.QueryRow(`SELECT name FROM app.role WHERE id = $1::uuid`, roleID).Scan(&name))
	assert.Equal(t, "Server Name", name)
}

func TestApply_ServerWins_RowAlreadyDeleted(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Deleted Role")
	_, err := conn.Exec(`UPDATE app.role SET deleted_at = now() WHERE id = $1::uuid`, roleID)
	require.NoError(t, err)

	// client tries to update a role the server has already deleted
	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{
			updateRoleCommit(ownerID, wsID, roleID, "Revived Name", time.Now().Add(time.Second)),
		},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Confirmed)
	require.Len(t, resp.Restored, 1)

	item := resp.Restored[0]
	assert.Equal(t, "role", item.TableName)

	payload, ok := item.ServerPayload.(map[string]interface{})
	require.True(t, ok)
	// server sends back the deleted row so client applies the delete locally
	assert.NotNil(t, payload["deleted_at"], "restored payload must include deleted_at")

	// row still soft-deleted on server
	var deletedAt *time.Time
	require.NoError(t, conn.QueryRow(`SELECT deleted_at FROM app.role WHERE id = $1::uuid`, roleID).Scan(&deletedAt))
	assert.NotNil(t, deletedAt)
}

func TestApply_UserToRole_InsertsRow(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID, utrID := newID(), newID(), newID(), newID()
	memberID := newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Admins")

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "INSERT",
			TableName:   "user_to_role",
			ObjectID:    utrID,
			WorkspaceID: wsID,
			UserID:      ownerID,
			CreatedAt:   time.Now(),
			Payload: map[string]any{
				"id":           utrID,
				"user_id":      memberID,
				"role_id":      roleID,
				"workspace_id": wsID,
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)

	var storedRoleID string
	require.NoError(t, conn.QueryRow(`SELECT role_id FROM app.user_to_role WHERE id = $1::uuid`, utrID).Scan(&storedRoleID))
	assert.Equal(t, roleID, storedRoleID)
}

func TestApply_Permission_NullableFieldsOmitted(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID, permID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Admins")

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "INSERT",
			TableName:   "permission",
			ObjectID:    permID,
			WorkspaceID: wsID,
			UserID:      ownerID,
			CreatedAt:   time.Now(),
			Payload: map[string]any{
				"id":      permID,
				"role_id": roleID,
				"action":  "select",
				"effect":  "allow",
				// db_instance_id, schema_name, table_name, column_name intentionally omitted
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)

	idUUID, err := db_types.NewJSONNullUUIDFromString(permID)
	require.NoError(t, err)
	wsUUID, err := db_types.NewJSONNullUUIDFromString(wsID)
	require.NoError(t, err)
	perm, err := db.Queries.GetPermissionByID(context.Background(), generated.GetPermissionByIDParams{ID: idUUID, WorkspaceID: wsUUID})
	require.NoError(t, err)
	assert.Equal(t, "select", perm.Action.String)
	assert.Equal(t, "allow", perm.Effect.String)
	assert.False(t, perm.DbInstanceID.Valid, "db_instance_id must be NULL when omitted")
	assert.False(t, perm.SchemaName.Valid, "schema_name must be NULL when omitted")
	assert.False(t, perm.TableName.Valid, "table_name must be NULL when omitted")
	assert.False(t, perm.ColumnName.Valid, "column_name must be NULL when omitted")
}

func TestApply_Permission_NullableFieldsStored(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID, permID := newID(), newID(), newID(), newID()
	dbInstanceID := newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Admins")

	resp, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          newID(),
			Operation:   "INSERT",
			TableName:   "permission",
			ObjectID:    permID,
			WorkspaceID: wsID,
			UserID:      ownerID,
			CreatedAt:   time.Now(),
			Payload: map[string]any{
				"id":             permID,
				"role_id":        roleID,
				"action":         "select",
				"effect":         "allow",
				"db_instance_id": dbInstanceID,
				"schema_name":    "public",
				"table_name":     "users",
				"column_name":    "email",
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Confirmed, 1)

	idUUID, err := db_types.NewJSONNullUUIDFromString(permID)
	require.NoError(t, err)
	wsUUID, err := db_types.NewJSONNullUUIDFromString(wsID)
	require.NoError(t, err)
	perm, err := db.Queries.GetPermissionByID(context.Background(), generated.GetPermissionByIDParams{ID: idUUID, WorkspaceID: wsUUID})
	require.NoError(t, err)
	assert.Equal(t, dbInstanceID, perm.DbInstanceID.String)
	assert.Equal(t, "public", perm.SchemaName.String)
	assert.Equal(t, "users", perm.TableName.String)
	assert.Equal(t, "email", perm.ColumnName.String)
}
