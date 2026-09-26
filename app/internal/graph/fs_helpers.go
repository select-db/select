package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"selectDb/internal/fs_uri"
	"selectDb/internal/utils"
)

// UserConfigDir returns the absolute path to the per-user config directory
// (where personal .theme / .config files live), ensuring it exists. It resolves
// from the same per-user app data directory used for server/workspace data
// (utils handles XDG / %APPDATA% per-OS), so personal config is shared across
// every server and workspace.
func UserConfigDir() (string, error) {
	return utils.UserConfigDir()
}

// WorkspaceFS encapsulates common path/URI computations for a single
// workspace so that both the initial graph build and the filesystem watcher
// share the exact same mapping logic.
type WorkspaceFS struct {
	WorkspaceID   string
	WorkspaceRoot string
	RootURI       string

	// ignore keeps the tree and the watcher out of node_modules and friends.
	ignore *ignoreMatcher
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
		RootURI:       fs_uri.Scheme + fs_uri.WorkspacePrefix + workspaceID,
		ignore:        newIgnoreMatcher(workspaceRoot),
	}
}

// Rel returns the slash-separated path of p relative to the workspace root and
// a boolean indicating whether p is inside the workspace.
func (c *WorkspaceFS) Rel(p string) (string, bool) {
	rel, err := filepath.Rel(c.WorkspaceRoot, p)
	if err != nil || !fs_uri.Contains(c.WorkspaceRoot, p) {
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

// Path converts a selectdb:// URI back to an absolute path, and reports whether
// the URI belongs to this workspace.
func (c *WorkspaceFS) Path(uri string) (string, bool) {
	if uri == c.RootURI {
		return c.WorkspaceRoot, true
	}
	rel, ok := strings.CutPrefix(uri, c.RootURI+"/")
	if !ok {
		return "", false
	}
	path, err := fs_uri.Resolve(c.WorkspaceRoot, rel)
	return path, err == nil
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

// DatasourceConfigFileName is the file that makes a directory a database.
const DatasourceConfigFileName = "datasource.config.json"

// FSDatasourceConfig mirrors the on-disk datasource.config.json structure used for
// filesystem-backed datasources managed by the workspace graph. It is a
// lightweight version of the fsDatasourceConfig type in node_datasource.go, kept here
// to avoid import cycles.
//
// The name is deliberately absent: a database is named by the directory it
// sits in, and a second copy here would be one nothing reads and every rename
// would leave behind.
type FSDatasourceConfig struct {
	ID        string                 `json:"id"`
	DbType    string                 `json:"db_type"`
	DSN       string                 `json:"dsn"`
	SSH       *FSDatasourceSSHConfig `json:"ssh,omitempty"`
	Proxified bool                   `json:"proxified,omitempty"`
}

// FSDatasourceSSHConfig is a minimal SSH configuration used in datasource.config.json for a DB
// instance. All sensitive values are expected to be provided via .env
// variables and referenced here using $VAR tokens.
type FSDatasourceSSHConfig struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	AuthMethod string `json:"auth_method"` // "password" | "private_key" | "agent" | "key_file"
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	KeyPath    string `json:"key_path"` // path to a private key file (desktop key_file auth)
	HostKey    string `json:"host_key"`
}

// SSHConfigFromFS maps a persisted FS SSH config to a graph-node SSH config.
// Single source of truth for both the full graph build and the incremental file
// watcher, so neither can silently drop a field (e.g. key_path / host_key).
// Returns nil when fs is nil.
func SSHConfigFromFS(fs *FSDatasourceSSHConfig) *DatasourceSSHConfig {
	if fs == nil {
		return nil
	}
	ssh := &DatasourceSSHConfig{
		Enabled:    fs.Enabled,
		Host:       fs.Host,
		Port:       fs.Port,
		User:       fs.User,
		AuthMethod: fs.AuthMethod,
		Password:   fs.Password,
		PrivateKey: fs.PrivateKey,
		KeyPath:    fs.KeyPath,
		HostKey:    fs.HostKey,
	}
	if ssh.Enabled && ssh.Port == 0 {
		ssh.Port = 22
	}
	return ssh
}

// ReadFSDatasourceConfig reads and unmarshals an FSDatasourceConfig from the given path.
func ReadFSDatasourceConfig(path string) (*FSDatasourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg FSDatasourceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DatasourceRef is a single database reference in file metadata.
type DatasourceRef struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// FileMetadata represents the sidecar metadata stored for files
// ("*.metadata.json").
type FileMetadata struct {
	Datasources []DatasourceRef `json:"datasources"`
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

	// Sidecars SELECT writes for its own use and nobody edits by hand.
	//
	// The two config files are not among them. datasource.config.json and
	// select.config.json are read, edited and committed by people -- one
	// carries the dialect and the $VAR a DSN resolves from, the other is what
	// a teammate clones to land in the same workspace -- so both are rows.
	if strings.HasSuffix(name, ".metadata.json") ||
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

// CheckIsDatasource checks if a directory contains a datasource.config.json file,
// indicating it's a datasource directory.
func CheckIsDatasource(dirPath string) bool {
	datasourceConfigPath := filepath.Join(dirPath, DatasourceConfigFileName)
	_, err := os.Stat(datasourceConfigPath)
	return err == nil
}
