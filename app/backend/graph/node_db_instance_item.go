package graph

import (
	"slices"

	"github.com/selectDb/toolkit"
)

type DBInstanceItemNode struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Type string `json:"type"`

	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Badges   []string    `json:"badges"`
	Metadata interface{} `json:"metadata"`

	ParentID string                `json:"parent_id"`
	Children []*DBInstanceItemNode `json:"children"`
}

func (dbi *DBInstanceItemNode) GetIDs() []string {
	return []string{dbi.ID}
}

func (dbi *DBInstanceItemNode) GetParentIDs() []string {
	return []string{dbi.ParentID}
}

func (dbi *DBInstanceItemNode) RemoveChildByIDs(IDs []string) bool {
	for i, child := range dbi.Children {
		if toolkit.Intersects(child.GetIDs(), IDs) {
			dbi.Children = slices.Delete(dbi.Children, i, i+1)
			return true
		}
	}
	return false
}

func (dbi *DBInstanceItemNode) GetChildren() []Node {
	nodes := make([]Node, 0, len(dbi.Children))
	for _, c := range dbi.Children {
		nodes = append(nodes, c)
	}
	return nodes
}

func (dbi *DBInstanceItemNode) AddChild(n Node) bool {
	switch node := n.(type) {
	case *DBInstanceItemNode:
		dbi.Children = append(dbi.Children, node)
	default:
		return false
	}
	return true
}
