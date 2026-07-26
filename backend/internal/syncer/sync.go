package syncer

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"backend/db"
	"backend/internal/syncer/gen"
	"backend/internal/syncer/types"
	"backend/internal/syncer/workspace"
)

// handlers is the dispatch table for every synced table: the generated registry
// (gen.Handlers, one entry per @app.sync table) plus the hand-written specials
// that stay outside generation — currently only workspace, which is its own
// tenant and can't follow the workspace-FK convention the generator relies on.
var handlers = func() map[string]gen.Handler {
	m := map[string]gen.Handler{
		"workspace": {Apply: workspace.Apply, ApplyDelete: workspace.ApplyDelete, FetchCurrent: workspace.FetchCurrent},
	}
	for k, v := range gen.Handlers {
		m[k] = v
	}
	return m
}()

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

		h, ok := handlers[c.TableName]
		if !ok {
			continue
		}

		var applied bool
		var rest *types.RestoredItem
		var applyErr error
		if c.Operation == "delete" {
			applied, rest, applyErr = h.ApplyDelete(ctx, userID, c)
		} else {
			applied, rest, applyErr = h.Apply(ctx, userID, c, lastPulledAt)
		}

		if applyErr != nil {
			return nil, false, fmt.Errorf("apply %s commit %s: %w", c.TableName, c.ID, applyErr)
		}

		if applied {
			// Run the hand-written auth side effects this write triggers (token
			// revocation, permission-cache invalidation; see side_effects.go) so
			// the change is enforced immediately, and signal the caller to refresh
			// their own token.
			applyCommitSideEffects(ctx, c)
			if c.TableName == "user_to_role" || c.TableName == "permission" ||
				c.TableName == "user_to_group" || c.TableName == "group_to_role" {
				needsTokenRefresh = true
			}
		}

		confirmed, restored, appliedIDs = recordResult(c, applied, rest, confirmed, restored, appliedIDs)
	}

	changes, err := getChangesSince(ctx, userID, lastPulledAt)
	if err != nil {
		return nil, false, fmt.Errorf("get changes since: %w", err)
	}

	if len(appliedIDs) > 0 {
		// workspace is the hand-written special; the @app.sync fields are filtered
		// by the generated gen.FilterApplied (see gen/changes.go).
		changes.Workspaces = gen.FilterByID(changes.Workspaces, appliedIDs, func(r types.WorkspaceRow) string { return r.ID })
		gen.FilterApplied(changes, appliedIDs)
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

	// workspace is the hand-written special (self-tenant, not @app.sync).
	workspaces, err := workspace.GetChangesSince(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	changes.Workspaces = workspaces

	// Every @app.sync table's rows, filled by the generated aggregation.
	if err := gen.FetchChanges(ctx, userID, since, changes); err != nil {
		return nil, err
	}

	return changes, nil
}

func fetchCurrentForUnauthorized(ctx context.Context, c types.Commit) (*types.RestoredItem, error) {
	h, ok := handlers[c.TableName]
	if !ok {
		return nil, nil
	}
	return h.FetchCurrent(ctx, c)
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
