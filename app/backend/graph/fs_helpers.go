package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"selectDb/backend/server"
)

// WorkspaceFS encapsulates common path/URI computations for a single
// workspace so that both the initial graph build and the filesystem watcher
// share the exact same mapping logic.
type WorkspaceFS struct {
	WorkspaceID   string
	WorkspaceRoot string
	RootURI       string
}

// WorkspaceRootPath returns the absolute filesystem path to the workspace root
// directory for the given workspace ID (under the current server folder).
func WorkspaceRootPath(workspaceID string) (string, error) {
	serverRoot, err := server.CurrentServerRoot()
	if err != nil {
		return "", err
	}
	if serverRoot == "" {
		return "", fmt.Errorf("no current server")
	}
	return filepath.Join(serverRoot, "workspaces", workspaceID), nil
}

// NewWorkspaceFS constructs a WorkspaceFS by resolving the workspace root on
// disk from the given workspace ID.
func NewWorkspaceFS(workspaceID string) (*WorkspaceFS, error) {
	root, err := WorkspaceRootPath(workspaceID)
	if err != nil {
		return nil, err
	}
	return NewWorkspaceFSFromRoot(workspaceID, root), nil
}

// NewWorkspaceFSFromRoot constructs a WorkspaceFS from a known workspace root
// path. This is useful when the caller already resolved the root directory.
func NewWorkspaceFSFromRoot(workspaceID, workspaceRoot string) *WorkspaceFS {
	return &WorkspaceFS{
		WorkspaceID:   workspaceID,
		WorkspaceRoot: workspaceRoot,
		RootURI:       fmt.Sprintf("selectdb://workspaces/%s", workspaceID),
	}
}

// Rel returns the slash-separated path of p relative to the workspace root and
// a boolean indicating whether p is inside the workspace.
func (c *WorkspaceFS) Rel(p string) (string, bool) {
	rel, err := filepath.Rel(c.WorkspaceRoot, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// URI converts a workspace-relative path to a selectdb:// URI.
func (c *WorkspaceFS) URI(rel string) string {
	if rel == "." || rel == "" {
		return c.RootURI
	}
	return c.RootURI + "/" + filepath.ToSlash(rel)
}

// ParentURI returns the URI of the parent folder for a given workspace-relative
// path. For items directly under the workspace root, it returns the root URI.
func (c *WorkspaceFS) ParentURI(rel string) string {
	parentRel := filepath.Dir(rel)
	if parentRel == "." || parentRel == "/" {
		return c.URI("")
	}
	return c.URI(parentRel)
}

// FSDBConfig mirrors the on-disk db.config.json structure used for
// filesystem-backed database instances managed by the workspace graph. It is a
// lightweight version of the fsDBConfig type in node_db_instance.go, kept here
// to avoid import cycles.
type FSDBConfig struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	DbType    string         `json:"db_type"`
	DSN       string         `json:"dsn"`
	SSH       *FSDBSSHConfig `json:"ssh,omitempty"`
	Proxified bool           `json:"proxified,omitempty"`
}

// FSDBSSHConfig is a minimal SSH configuration used in db.config.json for a DB
// instance. All sensitive values are expected to be provided via .env
// variables and referenced here using $VAR tokens.
type FSDBSSHConfig struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	AuthMethod string `json:"auth_method"` // "password" | "private_key"
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	HostKey    string `json:"host_key"`
}

// ReadFSDBConfig reads and unmarshals an FSDBConfig from the given path.
func ReadFSDBConfig(path string) (*FSDBConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg FSDBConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DatabaseRef is a single database reference in file metadata.
type DatabaseRef struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// FileMetadata represents the sidecar metadata stored for files
// ("*.metadata.json").
type FileMetadata struct {
	Databases []DatabaseRef `json:"databases"`
}

// ReadFileMetadata reads and unmarshals FileMetadata from the given path.
func ReadFileMetadata(path string) (*FileMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta FileMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// IsInternalWorkspaceFile reports whether a file name corresponds to an
// internal/config file that should not be exposed as a user-facing FileNode in
// the workspace graph.
func IsInternalWorkspaceFile(name string) bool {
	if name == "" {
		return false
	}

	lower := strings.ToLower(name)

	// Workspace-specific config / sidecar files.
	if name == "db.config.json" ||
		strings.HasSuffix(name, ".metadata.json") ||
		strings.HasPrefix(name, ".selectdb_") {
		return true
	}

	// Common OS/system artifacts we never want to show.
	if lower == ".ds_store" || lower == "thumbs.db" {
		return true
	}

	// macOS resource fork files.
	if strings.HasPrefix(name, "._") {
		return true
	}

	return false
}

// IsInternalWorkspacePath reports whether a workspace-relative path should be
// treated as internal/config and ignored by both the graph builder and the
// filesystem watcher. This is the path form returned by WorkspaceFS.Rel,
// using forward slashes.
func IsInternalWorkspacePath(rel string) bool {
	if rel == "" {
		return false
	}

	// Ignore Git repository internals entirely.
	if rel == ".git" ||
		strings.HasPrefix(rel, ".git/") ||
		strings.Contains(rel, "/.git/") {
		return true
	}

	return false
}

// CheckIsDBInstance checks if a directory contains a db.config.json file,
// indicating it's a database instance directory.
func CheckIsDBInstance(dirPath string) bool {
	dbConfigPath := filepath.Join(dirPath, "db.config.json")
	_, err := os.Stat(dbConfigPath)
	return err == nil
}
