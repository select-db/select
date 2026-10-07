package datasource

import (
	"context"
	"net/http"

	"backend/db"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"
	"backend/internal/datasource/managed"

	"github.com/google/uuid"
)

func DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		if idStr == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		actor := authz.ActorOf(r)
		workspaceID := actor.WorkspaceID

		if !actor.ManagesDatasource(idStr) {
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
		isManaged := err == nil && existing.State.ValueOrEmpty() != ""
		if isManaged {
			if err := managed.CheckAvailable(existing); err != nil {
				OpenError(w, err, "managed delete", workspaceID, idStr)
				return
			}
		}
		if err := deleteDatasource(r.Context(), parsedWorkspaceID, id); err != nil {
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

// deleteDatasource removes a datasource, its rules from every role, and the
// roles left with no rule anywhere else, all or nothing.
func deleteDatasource(ctx context.Context, workspaceID, datasourceID uuid.UUID) error {
	tx, err := db.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	queries := db.Queries.WithTx(tx)

	if err := queries.DeleteDatasource(ctx, generated.DeleteDatasourceParams{ID: datasourceID, WorkspaceID: workspaceID}); err != nil {
		return err
	}
	changedRoles, err := queries.DeleteDatasourceRules(ctx, generated.DeleteDatasourceRulesParams{WorkspaceID: workspaceID, DatasourceID: datasourceID.String()})
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, role := range changedRoles {
		authz.Invalidate(role.RoleID.String())
		if role.Dropped {
			audit.EmitChange(ctx, audit.RoleDeleted, workspaceID.String(), role.RoleID.String(), nil, nil)
		}
	}
	return nil
}
