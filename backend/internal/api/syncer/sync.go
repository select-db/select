package syncer

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"backend/db"
	"backend/internal/api/syncer/permission"
	"backend/internal/api/syncer/role"
	"backend/internal/api/syncer/types"
	"backend/internal/api/syncer/user_to_role"
	"backend/internal/api/syncer/workspace"
	"backend/internal/api/syncer/workspace_to_user"
)

// 1. Apply commits with last-write-wins
// 2. then return changes since last_pulled_at
func Sync(ctx context.Context, userID string, workspaceIDs []string, roleIDs []string, ownedWorkspaceIDs []string, req *types.SyncRequest) (*types.SyncResponse, bool, error) {
	if db.GetDB() == nil {
		return nil, false, nil
	}

	needsTokenRefresh := false
	var confirmed []string
	var restored []types.RestoredItem
	// Object IDs applied in this request: don't echo them back in changes.
	appliedIDs := make(map[string]struct{})

	lastPulledAt := time.Time{}
	if req.LastPulledAt != nil {
		lastPulledAt = *req.LastPulledAt
	}

	for _, c := range req.PendingCommits {
		if !authorizeCommit(userID, workspaceIDs, roleIDs, ownedWorkspaceIDs, c) {
			rest, err := fetchCurrentForUnauthorized(ctx, c)
			if err != nil {
				return nil, false, fmt.Errorf("fetch current for unauthorized commit %s: %w", c.ID, err)
			}
			if rest != nil {
				restored = append(restored, *rest)
			} else {
				// Row doesn't exist on server; confirm so client drops the pending commit.
				confirmed = append(confirmed, c.ID)
			}
			continue
		}

		var applied bool
		var rest *types.RestoredItem
		var applyErr error

		switch c.Operation {
		case "delete":
			switch c.TableName {
			case "workspace":
				applied, rest, applyErr = workspace.ApplyDelete(ctx, userID, c)
			case "workspace_to_user":
				applied, rest, applyErr = workspace_to_user.ApplyDelete(ctx, userID, c)
			case "role":
				applied, rest, applyErr = role.ApplyDelete(ctx, userID, c)
			case "user_to_role":
				applied, rest, applyErr = user_to_role.ApplyDelete(ctx, userID, c)
			case "permission":
				applied, rest, applyErr = permission.ApplyDelete(ctx, userID, c)
			default:
				continue
			}
		default:
			switch c.TableName {
			case "workspace":
				applied, rest, applyErr = workspace.Apply(ctx, userID, c, lastPulledAt)
			case "workspace_to_user":
				applied, rest, applyErr = workspace_to_user.Apply(ctx, userID, c, lastPulledAt)
			case "role":
				applied, rest, applyErr = role.Apply(ctx, userID, c, lastPulledAt)
			case "user_to_role":
				applied, rest, applyErr = user_to_role.Apply(ctx, userID, c, lastPulledAt)
			case "permission":
				applied, rest, applyErr = permission.Apply(ctx, userID, c, lastPulledAt)
			default:
				continue
			}
		}

		if applyErr != nil {
			return nil, false, fmt.Errorf("apply %s commit %s: %w", c.TableName, c.ID, applyErr)
		}

		if applied && (c.TableName == "user_to_role" || c.TableName == "permission") {
			needsTokenRefresh = true
		}

		confirmed, restored, appliedIDs = recordResult(c, applied, rest, confirmed, restored, appliedIDs)
	}

	changes, err := getChangesSince(ctx, userID, lastPulledAt)
	if err != nil {
		return nil, false, fmt.Errorf("get changes since: %w", err)
	}

	if len(appliedIDs) > 0 {
		changes.Workspaces = filterByID(changes.Workspaces, appliedIDs, func(r types.WorkspaceRow) string { return r.ID })
		changes.WorkspaceToUser = filterByID(changes.WorkspaceToUser, appliedIDs, func(r types.WorkspaceToUserRow) string { return r.ID })
		changes.Role = filterByID(changes.Role, appliedIDs, func(r types.RoleRow) string { return r.ID })
		changes.UserToRole = filterByID(changes.UserToRole, appliedIDs, func(r types.UserToRoleRow) string { return r.ID })
		changes.Permission = filterByID(changes.Permission, appliedIDs, func(r types.PermissionRow) string { return r.ID })
	}

	// Resolve related users so client can upsert users before workspace_to_user (FK).
	// TODO: create a generic approach for related fields
	if err := resolveRelatedUsers(ctx, db.GetDB(), changes); err != nil {
		return nil, false, fmt.Errorf("resolve related users: %w", err)
	}

	serverTime := time.Now().UTC()
	return &types.SyncResponse{
		Confirmed:  confirmed,
		Restored:   restored,
		Changes:    *changes,
		ServerTime: serverTime,
	}, needsTokenRefresh, nil
}

// Fills changes.Users only when needed: user_ids referenced by workspace_to_user
func resolveRelatedUsers(ctx context.Context, _ *sql.DB, changes *types.SyncChanges) error {
	if len(changes.WorkspaceToUser) == 0 {
		changes.Users = nil
		return nil
	}
	seen := make(map[string]struct{})
	for _, wtu := range changes.WorkspaceToUser {
		seen[wtu.UserID] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	rows, err := db.Queries.GetUsersByIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("get user names: %w", err)
	}
	changes.Users = nil
	for _, r := range rows {
		row := types.UserRow{ID: r.ID.String(), Email: r.Email, AvatarURL: r.AvatarUrl}
		if r.Name.Valid {
			row.Name = r.Name.ValueOrEmpty()
		}
		changes.Users = append(changes.Users, row)
	}
	return nil
}

// Returns changes for the user updated after since.
func getChangesSince(ctx context.Context, userID string, since time.Time) (*types.SyncChanges, error) {
	changes := &types.SyncChanges{}

	workspaces, err := workspace.GetChangesSince(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	changes.Workspaces = workspaces

	wtuList, err := workspace_to_user.GetChangesSince(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	changes.WorkspaceToUser = wtuList

	roles, err := role.GetChangesSince(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	changes.Role = roles

	userToRoles, err := user_to_role.GetChangesSince(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	changes.UserToRole = userToRoles

	permissions, err := permission.GetChangesSince(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	changes.Permission = permissions

	return changes, nil
}

func fetchCurrentForUnauthorized(ctx context.Context, c types.Commit) (*types.RestoredItem, error) {
	switch c.TableName {
	case "workspace":
		return workspace.FetchCurrent(ctx, c)
	case "workspace_to_user":
		return workspace_to_user.FetchCurrent(ctx, c)
	case "role":
		return role.FetchCurrent(ctx, c)
	case "user_to_role":
		return user_to_role.FetchCurrent(ctx, c)
	case "permission":
		return permission.FetchCurrent(ctx, c)
	}
	return nil, nil
}

func recordResult(c types.Commit, applied bool, rest *types.RestoredItem, confirmed []string, restored []types.RestoredItem, appliedIDs map[string]struct{}) ([]string, []types.RestoredItem, map[string]struct{}) {
	if applied {
		confirmed = append(confirmed, c.ID)
		appliedIDs[c.ObjectID] = struct{}{}
	} else if rest != nil {
		restored = append(restored, *rest)
	}
	return confirmed, restored, appliedIDs
}

// filterByID removes rows whose ID is in the applied set.
func filterByID[T any](rows []T, applied map[string]struct{}, id func(T) string) []T {
	filtered := rows[:0]
	for _, r := range rows {
		if _, ok := applied[id(r)]; !ok {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
