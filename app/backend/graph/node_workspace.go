package graph

import (
	"slices"

	"github.com/selectDb/toolkit"
)

type WorkspaceNode struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	Name    string `json:"name"`
	IsOwner bool   `json:"is_owner"`

	User        *UserNode         `json:"user"`
	Folders     []*FolderNode     `json:"folders"`
	DBInstances []*DBInstanceNode `json:"db_instances"`
}

func (wg *WorkspaceNode) GetIDs() []string {
	return []string{wg.ID}
}

func (wg *WorkspaceNode) GetParentIDs() []string {
	return []string{}
}

func (wg *WorkspaceNode) GetPath() string {
	return ""
}

func (wg *WorkspaceNode) RemoveChildByIDs(IDs []string) bool {
	for i, folder := range wg.Folders {
		if toolkit.Intersects(folder.GetIDs(), IDs) {
			wg.Folders = slices.Delete(wg.Folders, i, i+1)
			return true
		}
	}
	for i, db := range wg.DBInstances {
		if toolkit.Intersects(db.GetIDs(), IDs) {
			wg.DBInstances = slices.Delete(wg.DBInstances, i, i+1)
			return true
		}
	}
	return false
}

func (wg *WorkspaceNode) GetChildren() []Node {
	nodes := make([]Node, 0, len(wg.Folders)+len(wg.DBInstances))
	for _, f := range wg.Folders {
		nodes = append(nodes, f)
	}
	for _, db := range wg.DBInstances {
		nodes = append(nodes, db)
	}
	return nodes
}

func (wg *WorkspaceNode) AddChild(n Node) bool {
	switch node := n.(type) {
	case *FolderNode:
		wg.Folders = append(wg.Folders, node)
	case *DBInstanceNode:
		wg.DBInstances = append(wg.DBInstances, node)
	default:
		return false
	}
	return true
}
