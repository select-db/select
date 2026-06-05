package user

import (
	"context"
	"fmt"
	"selectDb/backend/db/generated"
)

func (u *User) GetCurrentUser() (generated.User, error) {
	ctx := context.Background()
	user, err := u.Queries.GetCurrentUser(ctx)
	if err != nil {
		return generated.User{}, fmt.Errorf("GetCurrentUser: failed to fetch user: %w", err)
	}
	return user, nil
}
