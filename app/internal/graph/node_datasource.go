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

func BuildDatasourceNode(dto DatasourceDTO) *DatasourceNode {
	folderID := dto.FolderID
	if *folderID == "" {
		folderID = utils.Ptr("root")
	}

	workspaceID := ""
	if dto.WorkspaceID != nil {
		workspaceID = *dto.WorkspaceID
	}

	children := dto.Children
	if children == nil {
		children = []*DatasourceItemNode{}
	}
	files := dto.Files
	if files == nil {
		files = []*FileNode{}
	}
	folders := dto.Folders
	if folders == nil {
		folders = []*FolderNode{}
	}

	var sshConfig *DatasourceSSHConfig
	if dto.SSH != nil {
		ssh := *dto.SSH
		// Default port to 22 if not set and SSH is enabled.
		if ssh.Enabled && ssh.Port == 0 {
			ssh.Port = 22
		}
		sshConfig = &ssh
	}

	proxified := false
	if dto.Proxified != nil {
		proxified = *dto.Proxified
	}

	return &DatasourceNode{
		ID:   *dto.ID,
		URI:  *dto.URI,
		Type: "datasource",

		Name:      *utils.DefaultIfNil(dto.Name, ""),
		DBType:    *utils.DefaultIfNil(dto.DBType, "postgresql"),
		DSN:       *utils.DefaultIfNil(dto.DSN, ""),
		Proxified: proxified,

		SSH: sshConfig,

		FolderID:    *folderID,
		WorkspaceID: workspaceID,

		Children: children,
		Files:    files,
		Folders:  folders,
	}
}

func (dto *DatasourceNode) GetIDs() []string {
	return []string{dto.ID, dto.URI}
}

func (dto *DatasourceNode) GetParentIDs() []string {
	return []string{dto.FolderID, dto.WorkspaceID}
}

func (dto *DatasourceNode) RemoveChildByIDs(IDs []string) bool {
	for i, child := range dto.Children {
		if toolkit.Intersects(child.GetIDs(), IDs) {
			dto.Children = slices.Delete(dto.Children, i, i+1)
			return true
		}
	}
	for i, file := range dto.Files {
		if toolkit.Intersects(file.GetIDs(), IDs) {
			dto.Files = slices.Delete(dto.Files, i, i+1)
			return true
		}
	}
	for i, folder := range dto.Folders {
		if toolkit.Intersects(folder.GetIDs(), IDs) {
			dto.Folders = slices.Delete(dto.Folders, i, i+1)
			return true
		}
	}
	return false
}

func (dto *DatasourceNode) GetChildren() []Node {
	nodes := make([]Node, 0, len(dto.Children)+len(dto.Files)+len(dto.Folders))
	for _, c := range dto.Children {
		nodes = append(nodes, c)
	}
	for _, f := range dto.Files {
		nodes = append(nodes, f)
	}
	for _, folder := range dto.Folders {
		nodes = append(nodes, folder)
	}
	return nodes
}

func (dto *DatasourceNode) AddChild(n Node) bool {
	switch node := n.(type) {
	case *DatasourceItemNode:
		dto.Children = append(dto.Children, node)
	case *FileNode:
		dto.Files = append(dto.Files, node)
	case *FolderNode:
		dto.Folders = append(dto.Folders, node)
	default:
		return false
	}
	return true
}
