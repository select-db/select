package workspace

import (
	"context"
	"database/sql"

	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

// Restore upserts the server-authoritative workspace row. It creates nothing on
// disk: a workspace has no folder here until somebody opens one.
func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	name := utils.MapGetString(payload, "name")
	if id == "" || name == "" {
		return nil
	}
	existedBefore := true
	existing, err := queries.GetWorkspaceByID(ctx, id)
	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}
		existedBefore = false
	}

	ownerID := db_types.JSONNullString{}
	if oid := utils.MapGetString(payload, "owner_id"); oid != "" {
		ownerID = db_types.NewJSONNullString(oid)
	}
	// Execution limits are server-authoritative once the backend manages them.
	// Until then a pulled row may omit these fields; in that case we keep the
	// values already stored locally (defaults for a brand-new workspace) instead
	// of resetting them.
	defaultTimeout := 30000
	defaultSize := 100
	if existedBefore {
		defaultTimeout = int(existing.StatementTimeoutMs)
		defaultSize = int(existing.MaxResultSizeMb)
	}
	statementTimeoutMs := utils.MapGetIntOr(payload, "statement_timeout_ms", defaultTimeout)
	maxResultSizeMB := utils.MapGetIntOr(payload, "max_result_size_mb", defaultSize)

	// Server-authoritative: the pulled value replaces whatever is stored locally.
	// An explicit null must land as null; only an absent key (a server predating
	// the column) leaves the local value alone.
	logo := db_types.JSONNullString{}
	if payloadLogo := utils.MapGetStringPtr(payload, "logo"); payloadLogo != nil {
		logo = db_types.NewJSONNullString(*payloadLogo)
	} else if _, present := payload["logo"]; !present && existedBefore {
		logo = existing.Logo
	}

	if err := queries.UpsertWorkspaceForSync(ctx, generated.UpsertWorkspaceForSyncParams{
		ID:                 id,
		Name:               name,
		OwnerID:            ownerID,
		StatementTimeoutMs: int64(statementTimeoutMs),
		MaxResultSizeMb:    int64(maxResultSizeMB),
		Logo:               logo,
	}); err != nil {
		return err
	}

	return nil
}
