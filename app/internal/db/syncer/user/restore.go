package user

import (
	"context"

	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

// Restore upserts the server-authoritative user row (called on LWW conflict or changes apply).
func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	name := utils.MapGetString(payload, "name")
	if id == "" {
		return nil
	}
	email := utils.MapGetString(payload, "email")
	avatarURL := utils.MapGetString(payload, "avatar_url")
	if err := queries.UpsertUserForSync(ctx, generated.UpsertUserForSyncParams{
		ID:        id,
		Name:      db_types.NewJSONNullString(name),
		Email:     db_types.NewJSONNullString(email),
		AvatarUrl: db_types.NewJSONNullString(avatarURL),
	}); err != nil {
		return err
	}
	if avatarURL != "" {
		_ = utils.FetchAndSaveAvatar(ctx, avatarURL, id)
	}
	return nil
}
