package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WorkspaceConfigFileName marks a directory as a workspace, the way
// db.config.json marks one as a database.
const WorkspaceConfigFileName = "select.config.json"

// workspaceConfigVersion is the format this build writes.
const workspaceConfigVersion = 1

// JSON has no comments, and this file sits in the user's own repository.
const workspaceConfigComment = "Managed by SELECT. Do not edit by hand."

// WorkspaceConfig is select.config.json. It is meant to be committed, so a
// teammate who clones the repository lands in the right workspace.
//
// The name is absent on purpose: the server owns it, as with db.config.json.
type WorkspaceConfig struct {
	Comment     string `json:"$comment"`
	Version     int    `json:"version"`
	Server      string `json:"server"`
	WorkspaceID string `json:"workspaceId"`
}

// ErrNoWorkspaceConfig means the folder has no config, which is the ordinary
// case on a first open rather than a failure.
var ErrNoWorkspaceConfig = errors.New("no workspace config in folder")

// WorkspaceConfigPath returns where the config file lives for a folder.
func WorkspaceConfigPath(folder string) string {
	return filepath.Join(folder, WorkspaceConfigFileName)
}

// IsWorkspaceFolder reports whether folder holds a workspace config, mirroring
// CheckIsDBInstance for database directories.
func IsWorkspaceFolder(folder string) bool {
	_, err := os.Stat(WorkspaceConfigPath(folder))
	return err == nil
}

// ReadWorkspaceConfig returns ErrNoWorkspaceConfig when there is no file, and a
// plain error when there is one but it is unusable. Callers must keep the two
// apart: offering to create a workspace over a corrupt config loses the one it
// named.
func ReadWorkspaceConfig(folder string) (*WorkspaceConfig, error) {
	path := WorkspaceConfigPath(folder)

	data, err := os.ReadFile(path) // #nosec G304 -- path is a folder the user picked, joined with a constant name
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoWorkspaceConfig
		}
		return nil, fmt.Errorf("read %s: %w", WorkspaceConfigFileName, err)
	}

	var cfg WorkspaceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", WorkspaceConfigFileName, err)
	}
	if cfg.WorkspaceID == "" || cfg.Server == "" {
		return nil, fmt.Errorf("%s names no workspace or no server", WorkspaceConfigFileName)
	}
	if cfg.Version > workspaceConfigVersion {
		return nil, fmt.Errorf("%s was written by a newer version of SELECT (format %d)", WorkspaceConfigFileName, cfg.Version)
	}

	return &cfg, nil
}

// WriteWorkspaceConfig replaces folder's config. Indented and newline-terminated
// because it is committed and read by people.
func WriteWorkspaceConfig(folder, server, workspaceID string) error {
	cfg := WorkspaceConfig{
		Comment:     workspaceConfigComment,
		Version:     workspaceConfigVersion,
		Server:      server,
		WorkspaceID: workspaceID,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", WorkspaceConfigFileName, err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(WorkspaceConfigPath(folder), data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", WorkspaceConfigFileName, err)
	}
	return nil
}

// RemoveWorkspaceConfig unmakes a workspace. The folder itself is untouched.
func RemoveWorkspaceConfig(folder string) error {
	if err := os.Remove(WorkspaceConfigPath(folder)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", WorkspaceConfigFileName, err)
	}
	return nil
}
