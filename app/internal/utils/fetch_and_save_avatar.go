package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func FetchAndSaveAvatar(ctx context.Context, url string, userID string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status downloading avatar: %s", resp.Status)
	}

	appDataDir, err := GetAppDataDir()
	if err != nil {
		return err
	}

	avatarPath := filepath.Join(appDataDir, "assets", "avatars", userID+"_avatar.png")

	err = os.MkdirAll(filepath.Dir(avatarPath), 0755)
	if err != nil {
		return err
	}

	out, err := os.Create(avatarPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	return err
}
