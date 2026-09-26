package graph

import (
	"slices"

	"github.com/selectDb/toolkit"
)

type DatasourceItemNode struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Type string `json:"type"`

	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Badges   []string    `json:"badges"`
	Metadata interface{} `json:"metadata"`

	ParentID string                `json:"parent_id"`
	Children []*DatasourceItemNode `json:"children"`
}

func (dbi *DatasourceItemNode) GetIDs() []string {
	return []string{dbi.ID}
}

func (dbi *DatasourceItemNode) GetParentIDs() []string {
	return []string{dbi.ParentID}
}

func (dbi *DatasourceItemNode) RemoveChildByIDs(IDs []string) bool {
	for i, child := range dbi.Children {
		if toolkit.Intersects(child.GetIDs(), IDs) {
			dbi.Children = slices.Delete(dbi.Children, i, i+1)
			return true
		}
	}
	return false
}

func (dbi *DatasourceItemNode) GetChildren() []Node {
	nodes := make([]Node, 0, len(dbi.Children))
	for _, c := range dbi.Children {
		nodes = append(nodes, c)
	}
	return nodes
}

func (dbi *DatasourceItemNode) AddChild(n Node) bool {
	switch node := n.(type) {
	case *DatasourceItemNode:
		dbi.Children = append(dbi.Children, node)
	default:
		return false
	}
	return true
}
