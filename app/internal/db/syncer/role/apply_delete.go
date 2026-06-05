package role

import (
	"context"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

func ApplyDelete(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	if id == "" {
		return nil
	}
	return queries.DeleteRole(ctx, id)
}
