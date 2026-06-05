package syncer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"selectDb/internal/api"
	"selectDb/internal/db/generated"
	syncpermission "selectDb/internal/db/syncer/permission"
	syncrole "selectDb/internal/db/syncer/role"
	syncuser "selectDb/internal/db/syncer/user"
	syncutr "selectDb/internal/db/syncer/user_to_role"
	syncworkspace "selectDb/internal/db/syncer/workspace"
	syncwtu "selectDb/internal/db/syncer/workspace_to_user"
	"selectDb/internal/git"
	"selectDb/internal/utils"
)

const pendingCommitsBatchSize = 100

// normalizePayloadForSync ensures payload is sent as a JSON object. The DB may return payload as a string or []byte;
// the backend expects an object, so we decode to interface{} when needed.
func normalizePayloadForSync(payload interface{}) interface{} {
	var raw []byte
	switch v := payload.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return payload
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return payload
	}
	return out
}

// Only the current workspace is used for pending commits and last_pulled_at.
func (s *Syncer) Sync(ctx context.Context, userID string) error {
	currentWTU, err := s.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		// No current workspace: pull the backend's default
		// workspace so the next Sync call uses the normal path.
		return s.syncWith(ctx, userID, nil, nil, false, "")
	}

	var lastPulledAt *time.Time
	if at, err := s.Queries.GetLastPulledAtForWorkspace(ctx, currentWTU.WorkspaceID); err == nil && at.Valid {
		lastPulledAt = &at.Time
	}

	commits, err := s.Queries.GetPendingCommitsByUserAndWorkspace(ctx, generated.GetPendingCommitsByUserAndWorkspaceParams{
		UserID: userID, WorkspaceID: currentWTU.WorkspaceID, Limit: pendingCommitsBatchSize,
	})
	if err != nil {
		return fmt.Errorf("get pending commits: %w", err)
	}
	for {
		if len(commits) == 0 {
			return nil
		}
		if err := s.syncWith(ctx, userID, commits, lastPulledAt, true, currentWTU.WorkspaceID); err != nil {
			return err
		}
		if len(commits) < pendingCommitsBatchSize {
			return nil
		}
		commits, err = s.Queries.GetPendingCommitsByUserAndWorkspace(ctx, generated.GetPendingCommitsByUserAndWorkspaceParams{
			UserID: userID, WorkspaceID: currentWTU.WorkspaceID, Limit: pendingCommitsBatchSize,
		})
		if err != nil {
			return fmt.Errorf("get pending commits: %w", err)
		}
	}
}

// Pull fetches server-side changes without pushing any local commits.
// Use this after server-side operations (e.g. /user/add) to pull new records locally.
func (s *Syncer) Pull(ctx context.Context, userID string) error {
	currentWTU, err := s.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		return s.syncWith(ctx, userID, nil, nil, false, "")
	}

	var lastPulledAt *time.Time
	if at, err := s.Queries.GetLastPulledAtForWorkspace(ctx, currentWTU.WorkspaceID); err == nil && at.Valid {
		lastPulledAt = &at.Time
	}

	return s.syncWith(ctx, userID, nil, lastPulledAt, true, currentWTU.WorkspaceID)
}

// 1. sends commits to the backend
// 2. applies the response (confirmed, restored, changes)
// 3. update last_pulled_at
func (s *Syncer) syncWith(ctx context.Context, userID string, commits []generated.MutationCommit, lastPulledAt *time.Time, hadCurrentWorkspace bool, currentWorkspaceID string) error {
	pending := make([]SyncCommit, 0, len(commits))
	for _, c := range commits {
		pending = append(pending, SyncCommit{
			ID:          c.ID,
			CreatedAt:   c.CreatedAt,
			Operation:   c.Operation,
			TableName:   c.TableName,
			ObjectID:    c.ObjectID,
			Payload:     normalizePayloadForSync(c.Payload),
			UserID:      c.UserID,
			WorkspaceID: c.WorkspaceID,
		})
	}

	fetch := s.FetchFunc // UT function
	if fetch == nil {
		fetch = api.Fetch
	}

	var res SyncResponse
	if err := fetch(ctx, "POST", "sync/v1/sync", SyncRequest{
		PendingCommits: pending,
		LastPulledAt:   lastPulledAt,
	}, nil, &res); err != nil {
		return err
	}

	if err := s.deleteConfirmed(ctx, res.Confirmed); err != nil {
		return err
	}

	if err := s.applyRestored(ctx, res.Restored); err != nil {
		return err
	}

	if err := s.applyChanges(ctx, res.Changes); err != nil {
		return err
	}

	if s.EmitRolesUpdated != nil {
		c := res.Changes
		rolesAffected := len(c.Role) > 0 || len(c.UserToRole) > 0 || len(c.Permission) > 0
		if !rolesAffected {
			for _, r := range res.Restored {
				if r.TableName == "role" || r.TableName == "user_to_role" || r.TableName == "permission" {
					rolesAffected = true
					break
				}
			}
		}
		if rolesAffected {
			s.EmitRolesUpdated()
		}
	}

	if !hadCurrentWorkspace {
		if err := syncwtu.SetFirstAsCurrent(ctx, s.Queries, userID, res.Changes.WorkspaceToUser); err != nil {
			return err
		}
	}

	s.reconcileCurrentWorkspaceRepo(ctx)

	return s.updateLastPulledAt(ctx, currentWorkspaceID, res.ServerTime)
}

// reconcileCurrentWorkspaceRepo makes the current workspace's local repository
// converge to its server-authoritative git_remote_url. It runs on every sync,
// so a transient failure simply retries on the next sync instead of leaving the
// client desynced. Errors are intentionally non-fatal: a temporarily
// unreachable repo must not block role/permission sync.
func (s *Syncer) reconcileCurrentWorkspaceRepo(ctx context.Context) {
	if s.Git == nil {
		return
	}
	wtu, err := s.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		return
	}
	ws, err := s.Queries.GetWorkspaceByID(ctx, wtu.WorkspaceID)
	if err != nil {
		return
	}

	res, err := s.Git.ReconcileWorkspaceRemote(wtu.WorkspaceID, ws.GitRemoteUrl.Ptr())
	if err != nil {
		return
	}

	notable := res.Action == git.ReconcileSwitched ||
		res.Action == git.ReconcileUnlinked ||
		(res.Action == git.ReconcileLinked && res.Changed)
	if !notable {
		return
	}

	if res.Changed && s.Graph != nil {
		_ = s.Graph.RebuildWorkspaceGraph()
	}
	if s.EmitWorkspaceRepoChanged != nil {
		s.EmitWorkspaceRepoChanged(res)
	}
}

func (s *Syncer) deleteConfirmed(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := s.Queries.DeleteConfirmedCommits(ctx, ids); err != nil {
		return fmt.Errorf("delete confirmed commits: %w", err)
	}
	return nil
}

func (s *Syncer) updateLastPulledAt(ctx context.Context, workspaceID string, serverTime time.Time) error {
	if workspaceID == "" {
		return nil
	}
	if serverTime.IsZero() {
		serverTime = time.Now().UTC()
	}
	if err := s.Queries.UpdateWorkspacesLastPulledAt(ctx, generated.UpdateWorkspacesLastPulledAtParams{
		LastPulledAt: sql.NullTime{Time: serverTime, Valid: true},
		WorkspaceIds: []string{workspaceID},
	}); err != nil {
		return fmt.Errorf("update last_pulled_at: %w", err)
	}
	return nil
}

func (s *Syncer) applyRestored(ctx context.Context, items []SyncRestoredItem) error {
	for _, r := range items {
		payload, ok := r.ServerPayload.(map[string]any)
		if !ok {
			continue
		}
		if err := s.applyOneRow(ctx, r.TableName, payload); err != nil {
			return fmt.Errorf("apply restored %s %s: %w", r.TableName, r.ObjectID, err)
		}
	}
	return nil
}

// applyChanges upserts server-pushed rows in FK order: users → workspaces → workspace_to_user → roles → user_to_role → permissions.
// Rows with deleted_at set are applied as local deletes (and switch-or-logout if current).
func (s *Syncer) applyChanges(ctx context.Context, changes SyncChanges) error {
	for _, u := range changes.Users {
		if err := s.applyRow(ctx, "user", u); err != nil {
			return fmt.Errorf("upsert user: %w", err)
		}
	}
	for _, w := range changes.Workspaces {
		if err := s.applyOneRow(ctx, "workspace", w); err != nil {
			return fmt.Errorf("apply workspace: %w", err)
		}
	}
	for _, wtu := range changes.WorkspaceToUser {
		if err := s.applyOneRow(ctx, "workspace_to_user", wtu); err != nil {
			return fmt.Errorf("apply workspace_to_user: %w", err)
		}
	}
	for _, r := range changes.Role {
		if err := s.applyOneRow(ctx, "role", r); err != nil {
			return fmt.Errorf("apply role: %w", err)
		}
	}
	for _, utr := range changes.UserToRole {
		if err := s.applyOneRow(ctx, "user_to_role", utr); err != nil {
			return fmt.Errorf("apply user_to_role: %w", err)
		}
	}
	for _, p := range changes.Permission {
		if err := s.applyOneRow(ctx, "permission", p); err != nil {
			return fmt.Errorf("apply permission: %w", err)
		}
	}
	return nil
}

// applyOneRow applies a single server row, delete if has deleted_at, upsert otherwise.
func (s *Syncer) applyOneRow(ctx context.Context, tableName string, payload map[string]any) error {
	if PayloadHasDeletedAt(payload) {
		return s.applyDeleteRow(ctx, tableName, payload)
	}
	return s.applyRow(ctx, tableName, payload)
}

// applyDeleteRow dispatches to the table-specific ApplyDelete and runs switch-or-logout when the deleted row was current.
func (s *Syncer) applyDeleteRow(ctx context.Context, tableName string, payload map[string]any) error {
	var wasCurrent bool
	var userIDForSwitch string
	var err error
	switch tableName {
	case "workspace":
		wasCurrent, userIDForSwitch, err = syncworkspace.ApplyDelete(ctx, s.Queries, payload)
	case "workspace_to_user":
		wasCurrent, userIDForSwitch, err = syncwtu.ApplyDelete(ctx, s.Queries, payload)
	case "role":
		err = syncrole.ApplyDelete(ctx, s.Queries, payload)
	case "user_to_role":
		err = syncutr.ApplyDelete(ctx, s.Queries, payload)
	case "permission":
		err = syncpermission.ApplyDelete(ctx, s.Queries, payload)
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if tableName == "workspace" && s.Workspace != nil {
		if workspaceID := utils.MapGetString(payload, "id"); workspaceID != "" {
			_ = s.Workspace.RemoveWorkspaceFolderByID(workspaceID)
		}
	}
	if wasCurrent && userIDForSwitch != "" && s.SwitchOrLogout != nil {
		s.runSwitchOrLogout(ctx, userIDForSwitch)
	}
	return nil
}

// runSwitchOrLogout lists remaining workspaces for the user; if any, sets first as current and notifies; else logs out.
// Returns true if the user was logged out (no workspaces left).
func (s *Syncer) runSwitchOrLogout(ctx context.Context, userID string) bool {
	remaining, err := s.Queries.ListWorkspacesByUserID(ctx, userID)
	if err != nil || len(remaining) == 0 {
		if s.SwitchOrLogout != nil {
			s.SwitchOrLogout.OnLogout()
		}
		return true
	}
	// Set first remaining as current.
	if err := s.Queries.ClearCurrentWorkspaceToUser(ctx); err != nil {
		return false
	}
	first := remaining[0]
	if err := s.Queries.UpdateCurrentWorkspaceToUser(ctx, generated.UpdateCurrentWorkspaceToUserParams{
		UserID:      userID,
		WorkspaceID: first.ID,
	}); err != nil {
		return false
	}
	if s.Workspace != nil {
		_ = s.Workspace.EnsureWorkspaceFolderByID(first.ID, first.Name)
	}
	if s.Graph != nil {
		_ = s.Graph.RebuildWorkspaceGraph()
	}
	if s.SwitchOrLogout != nil {
		s.SwitchOrLogout.OnAfterWorkspaceSwitch()
	}
	return false
}

// applyRow upserts one server-authoritative row by table name.
func (s *Syncer) applyRow(ctx context.Context, tableName string, payload map[string]any) error {
	switch tableName {
	case "user":
		return syncuser.Restore(ctx, s.Queries, payload)
	case "workspace":
		return syncworkspace.Restore(ctx, s.Queries, payload, s.Workspace)
	case "workspace_to_user":
		return syncwtu.Restore(ctx, s.Queries, payload)
	case "role":
		return syncrole.Restore(ctx, s.Queries, payload)
	case "user_to_role":
		return syncutr.Restore(ctx, s.Queries, payload)
	case "permission":
		return syncpermission.Restore(ctx, s.Queries, payload)
	default:
		return nil
	}
}
