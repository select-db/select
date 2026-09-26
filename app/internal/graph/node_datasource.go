package graph

import (
	"selectDb/internal/utils"
	"slices"

	"github.com/selectDb/toolkit"
)

type DatasourceDTO struct {
	ID  *string `json:"id,omitempty"`
	URI *string `json:"uri,omitempty"`

	Name      *string `json:"name,omitempty"`
	DBType    *string `json:"db_type,omitempty"`
	DSN       *string `json:"dsn,omitempty"`
	Proxified *bool   `json:"proxified,omitempty"`

	SSH *DatasourceSSHConfig `json:"ssh,omitempty"`

	FolderID    *string               `json:"folder_id,omitempty"`
	WorkspaceID *string               `json:"workspace_id,omitempty"`
	Children    []*DatasourceItemNode `json:"children"`
	Files       []*FileNode           `json:"files,omitempty"`
	Folders     []*FolderNode         `json:"folders,omitempty"`
}

type DatasourceNode struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Type string `json:"type"`

	Name      string `json:"name"`
	DBType    string `json:"db_type"`
	DSN       string `json:"dsn"`
	Proxified bool   `json:"proxified,omitempty"`

	SSH *DatasourceSSHConfig `json:"ssh,omitempty"`

	FolderID    string `json:"folder_id"`
	WorkspaceID string `json:"workspace_id"`

	Children []*DatasourceItemNode `json:"children"`
	Files    []*FileNode           `json:"files"`
	Folders  []*FolderNode         `json:"folders"`
}

// DatasourceSSHConfig describes SSH tunneling configuration for a datasource.
// All sensitive values are expected to be provided via .env variables and
// referenced using $VAR tokens.
type DatasourceSSHConfig struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	AuthMethod string `json:"auth_method"` // "password" | "private_key" | "agent" | "key_file"
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	KeyPath    string `json:"key_path"` // path to a private key file (desktop key_file auth)
	HostKey    string `json:"host_key"` // pinned bastion public key
}

func BuildDatasourceNode(dbi DatasourceDTO) *DatasourceNode {
	folderID := dbi.FolderID
	if *folderID == "" {
		folderID = utils.Ptr("root")
	}

	workspaceID := ""
	if dbi.WorkspaceID != nil {
		workspaceID = *dbi.WorkspaceID
	}

	children := dbi.Children
	if children == nil {
		children = []*DatasourceItemNode{}
	}
	files := dbi.Files
	if files == nil {
		files = []*FileNode{}
	}
	folders := dbi.Folders
	if folders == nil {
		folders = []*FolderNode{}
	}

	var sshConfig *DatasourceSSHConfig
	if dbi.SSH != nil {
		ssh := *dbi.SSH
		// Default port to 22 if not set and SSH is enabled.
		if ssh.Enabled && ssh.Port == 0 {
			ssh.Port = 22
		}
		sshConfig = &ssh
	}

	proxified := false
	if dbi.Proxified != nil {
		proxified = *dbi.Proxified
	}

	return &DatasourceNode{
		ID:   *dbi.ID,
		URI:  *dbi.URI,
		Type: "datasource",

		Name:      *utils.DefaultIfNil(dbi.Name, ""),
		DBType:    *utils.DefaultIfNil(dbi.DBType, "postgresql"),
		DSN:       *utils.DefaultIfNil(dbi.DSN, ""),
		Proxified: proxified,

		SSH: sshConfig,

		FolderID:    *folderID,
		WorkspaceID: workspaceID,

		Children: children,
		Files:    files,
		Folders:  folders,
	}
}

func (dbi *DatasourceNode) GetIDs() []string {
	return []string{dbi.ID, dbi.URI}
}

func (dbi *DatasourceNode) GetParentIDs() []string {
	return []string{dbi.FolderID, dbi.WorkspaceID}
}

func (dbi *DatasourceNode) RemoveChildByIDs(IDs []string) bool {
	for i, child := range dbi.Children {
		if toolkit.Intersects(child.GetIDs(), IDs) {
			dbi.Children = slices.Delete(dbi.Children, i, i+1)
			return true
		}
	}
	for i, file := range dbi.Files {
		if toolkit.Intersects(file.GetIDs(), IDs) {
			dbi.Files = slices.Delete(dbi.Files, i, i+1)
			return true
		}
	}
	for i, folder := range dbi.Folders {
		if toolkit.Intersects(folder.GetIDs(), IDs) {
			dbi.Folders = slices.Delete(dbi.Folders, i, i+1)
			return true
		}
	}
	return false
}

func (dbi *DatasourceNode) GetChildren() []Node {
	nodes := make([]Node, 0, len(dbi.Children)+len(dbi.Files)+len(dbi.Folders))
	for _, c := range dbi.Children {
		nodes = append(nodes, c)
	}
	for _, f := range dbi.Files {
		nodes = append(nodes, f)
	}
	for _, folder := range dbi.Folders {
		nodes = append(nodes, folder)
	}
	return nodes
}

func (dbi *DatasourceNode) AddChild(n Node) bool {
	switch node := n.(type) {
	case *DatasourceItemNode:
		dbi.Children = append(dbi.Children, node)
	case *FileNode:
		dbi.Files = append(dbi.Files, node)
	case *FolderNode:
		dbi.Folders = append(dbi.Folders, node)
	default:
		return false
	}
	return true
}
