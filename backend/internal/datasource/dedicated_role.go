package datasource

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/authz"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/core"
)

// GrantTo names who gets a new managed database's dedicated role. Empty means
// nobody, as for any datasource.
type GrantTo struct {
	Users   []string `json:"users"`
	APIKeys []string `json:"api_keys"`
}

// fullAccess is every action the dedicated role of a managed database allows.
var fullAccess = []string{core.ActionSee, core.ActionSelect, core.ActionInsert, core.ActionUpdate, core.ActionDelete, core.ActionManage}

// resolveGrants parses and checks who gets the new role. The caller may always
// grant itself; granting anyone else takes the right to manage users or API keys.
func resolveGrants(ctx context.Context, actor authz.Actor, workspaceID uuid.UUID, grants GrantTo) (userIDs, apiKeyIDs []uuid.UUID, err error) {
	for _, rawID := range grants.Users {
		userID, err := uuid.Parse(rawID)
		if err != nil {
			return nil, nil, &Refusal{http.StatusBadRequest, "invalid user id in grant_to"}
		}
		isCaller := !actor.IsAPIKey && rawID == actor.UserID
		if !isCaller && !actor.IsOwner() && !actor.Can(core.ActionWorkspaceUsersManage) {
			return nil, nil, errForbidden
		}
		isMember, err := db.Queries.IsWorkspaceMember(ctx, generated.IsWorkspaceMemberParams{WorkspaceID: workspaceID, UserID: userID})
		if err != nil {
			return nil, nil, err
		}
		if !isMember {
			return nil, nil, &Refusal{http.StatusBadRequest, "grant_to names a user who is not a member of this workspace"}
		}
		userIDs = append(userIDs, userID)
	}
	for _, rawID := range grants.APIKeys {
		apiKeyID, err := uuid.Parse(rawID)
		if err != nil {
			return nil, nil, &Refusal{http.StatusBadRequest, "invalid api key id in grant_to"}
		}
		// An API key caller's UserID is its key id.
		isCaller := actor.IsAPIKey && rawID == actor.UserID
		if !isCaller && !actor.IsOwner() && !actor.Can(core.ActionWorkspaceApiKeysManage) {
			return nil, nil, errForbidden
		}
		_, err = db.Queries.GetAPIKeyForWorkspace(ctx, generated.GetAPIKeyForWorkspaceParams{ID: apiKeyID, WorkspaceID: workspaceID})
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, &Refusal{http.StatusBadRequest, "grant_to names an api key of another workspace"}
		}
		if err != nil {
			return nil, nil, err
		}
		apiKeyIDs = append(apiKeyIDs, apiKeyID)
	}
	return userIDs, apiKeyIDs, nil
}

// createDedicatedRole gives a managed database a role with full access on it,
// held by userIDs and apiKeyIDs. Its rows reach clients through sync like any other.
func createDedicatedRole(ctx context.Context, queries *generated.Queries, workspaceID uuid.UUID, datasourceID, name string, userIDs, apiKeyIDs []uuid.UUID) (uuid.UUID, error) {
	roleID := uuid.New()
	if err := queries.UpsertRole(ctx, generated.UpsertRoleParams{ID: roleID, WorkspaceID: workspaceID, Name: name}); err != nil {
		return roleID, err
	}
	for _, action := range fullAccess {
		if err := queries.UpsertPermission(ctx, generated.UpsertPermissionParams{
			ID:           uuid.New(),
			RoleID:       roleID,
			WorkspaceID:  workspaceID,
			DatasourceID: db_types.NewJSONNullString(datasourceID),
			Action:       action,
			Effect:       "allow",
		}); err != nil {
			return roleID, err
		}
	}
	for _, userID := range userIDs {
		if err := queries.UpsertUserToRole(ctx, generated.UpsertUserToRoleParams{ID: uuid.New(), UserID: userID, RoleID: roleID, WorkspaceID: workspaceID}); err != nil {
			return roleID, err
		}
	}
	for _, apiKeyID := range apiKeyIDs {
		if err := queries.AddAPIKeyRole(ctx, generated.AddAPIKeyRoleParams{ApiKeyID: apiKeyID, RoleID: roleID}); err != nil {
			return roleID, err
		}
	}
	return roleID, nil
}
