package user

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"selectDb/internal/utils"
)

func (u *User) GetCurrentUserAvatar(userID string) (string, error) {
	appDataDir, err := utils.GetAppDataDir()
	if err != nil {
		return "", err
	}

	avatarPath := filepath.Join(appDataDir, "assets", "avatars", userID+"_avatar.png")

	data, err := os.ReadFile(avatarPath)
	if err != nil {
		return "", fmt.Errorf("failed to read avatar: %w", err)
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	return "data:image/png;base64," + base64Data, nil
}
