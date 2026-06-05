package graph

import (
	"selectDb/internal/utils"
	"slices"

	"github.com/selectDb/toolkit"
)

type FolderDTO struct {
	ID  *string `json:"id,omitempty"`
	URI *string `json:"uri,omitempty"`

	Name *string `json:"name,omitempty"`

	FolderID *string `json:"folder_id,omitempty"`
}

type FolderNode struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Type string `json:"type"`

	Name string `json:"name"`

	FolderID string `json:"folder_id"`

	Files       []*FileNode       `json:"files"`
	Folders     []*FolderNode     `json:"folders"`
	DBInstances []*DBInstanceNode `json:"db_instances"`

	Variables map[string]string `json:"variables,omitempty"`

	Badges []string `json:"badges"`
}

func BuildFolderNode(f FolderDTO) *FolderNode {
	defaultFolder := "root"

	folderID := f.FolderID
	if *folderID == "" {
		folderID = utils.Ptr(defaultFolder)
	}

	return &FolderNode{
		ID:   *f.ID,
		URI:  *f.URI,
		Type: "folder",

		Name: *utils.DefaultIfNil(f.Name, ""),

		FolderID: *folderID,

		Files:       []*FileNode{},
		Folders:     []*FolderNode{},
		DBInstances: []*DBInstanceNode{},

		Variables: make(map[string]string),
	}
}

func (f *FolderNode) GetIDs() []string {
	return []string{f.ID}
}

func (f *FolderNode) GetParentIDs() []string {
	return []string{f.FolderID}
}

func (f *FolderNode) GetChildren() []Node {
	nodes := make([]Node, 0, len(f.Folders)+len(f.Files)+len(f.DBInstances))
	for _, subFolder := range f.Folders {
		nodes = append(nodes, subFolder)
	}
	for _, file := range f.Files {
		nodes = append(nodes, file)
	}
	for _, db := range f.DBInstances {
		nodes = append(nodes, db)
	}
	return nodes
}

func (f *FolderNode) RemoveChildByIDs(IDs []string) bool {
	for i, subFolder := range f.Folders {
		if toolkit.Intersects(subFolder.GetIDs(), IDs) {
			f.Folders = slices.Delete(f.Folders, i, i+1)
			return true
		}
	}
	for i, file := range f.Files {
		if toolkit.Intersects(file.GetIDs(), IDs) {
			f.Files = slices.Delete(f.Files, i, i+1)
			return true
		}
	}
	for i, db := range f.DBInstances {
		if toolkit.Intersects(db.GetIDs(), IDs) {
			f.DBInstances = slices.Delete(f.DBInstances, i, i+1)
			return true
		}
	}
	return false
}

func (f *FolderNode) AddChild(n Node) bool {
	switch node := n.(type) {
	case *FileNode:
		f.Files = append(f.Files, node)
	case *FolderNode:
		f.Folders = append(f.Folders, node)
	case *DBInstanceNode:
		f.DBInstances = append(f.DBInstances, node)
	default:
		return false
	}
	return true
}
