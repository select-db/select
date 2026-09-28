package datasource

import (
	"context"
	"net/http"

	"backend/db"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"

	"github.com/google/uuid"
)

func DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		if idStr == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		a := authz.ActorOf(r)
		workspaceID := a.WorkspaceID

		if !a.IsOwner() && !a.CanManage(idStr) {
			audit.EmitDenied(r.Context(), audit.DatasourceDeleted, workspaceID, idStr)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		parsedWorkspaceID, err := uuid.Parse(workspaceID)
		if err != nil {
			http.Error(w, "invalid workspace_id", http.StatusBadRequest)
			return
		}

		existing, err := db.Queries.GetDatasource(r.Context(), generated.GetDatasourceParams{ID: id, WorkspaceID: parsedWorkspaceID})
		isManaged := err == nil && existing.CellarID.ValueOrEmpty() != ""
		if isManaged {
			if err := checkServing(existing); err != nil {
				OpenError(w, err, "managed delete", workspaceID, idStr)
				return
			}
			err = deleteManaged(r.Context(), parsedWorkspaceID, idStr)
		} else {
			err = db.Queries.DeleteDatasource(r.Context(), generated.DeleteDatasourceParams{
				ID:          id,
				WorkspaceID: parsedWorkspaceID,
			})
		}
		if err != nil {
			http.Error(w, "failed to delete datasource", http.StatusInternalServerError)
			return
		}

		InvalidateCache(workspaceID, idStr)

		audit.EmitAction(r.Context(), audit.DatasourceDeleted, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    idStr,
			Status:      audit.StatusSuccess,
			Payload:     map[string]any{"managed": isManaged},
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// deleteManaged stops serving a managed database at once and leaves its file
// to the reconciler. Its rules go from every role, and a role left with no
// rule anywhere else goes with them.
func deleteManaged(ctx context.Context, workspaceID uuid.UUID, datasourceID string) error {
	tx, err := db.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	queries := db.Queries.WithTx(tx)

	if err := queries.MarkDatasourceDeleting(ctx, generated.MarkDatasourceDeletingParams{ID: uuid.MustParse(datasourceID), WorkspaceID: workspaceID}); err != nil {
		return err
	}
	dedicatedRoleIDs, err := queries.ListRolesScopedToDatasource(ctx, generated.ListRolesScopedToDatasourceParams{WorkspaceID: workspaceID, DatasourceID: datasourceID})
	if err != nil {
		return err
	}
	for _, roleID := range dedicatedRoleIDs {
		if err := queries.SetRoleDeletedAt(ctx, generated.SetRoleDeletedAtParams{ID: roleID, WorkspaceID: workspaceID}); err != nil {
			return err
		}
	}
	changedRoleIDs, err := queries.DeleteDatasourcePermissions(ctx, generated.DeleteDatasourcePermissionsParams{WorkspaceID: workspaceID, DatasourceID: datasourceID})
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, roleID := range dedicatedRoleIDs {
		audit.EmitChange(ctx, audit.RoleDeleted, workspaceID.String(), roleID.String(), nil, nil)
	}
	for _, roleID := range changedRoleIDs {
		authz.Invalidate(roleID.String())
	}
	return nil
}
