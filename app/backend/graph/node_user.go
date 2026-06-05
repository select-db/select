package graph

import "selectDb/backend/db/generated"

type UserNode struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	Name string `json:"name"`
}

func BuildUserNode(u generated.User) *UserNode {
	return &UserNode{
		ID:   u.ID,
		Type: "user",
		Name: u.Name.String,
	}
}

func (u *UserNode) GetIDs() []string {
	return []string{u.ID}
}

func (u *UserNode) GetParentIDs() []string {
	return []string{}
}

func (u *UserNode) RemoveChildByIDs(IDs []string) bool {
	return false
}

func (u *UserNode) GetChildren() []Node {
	return []Node{}
}

func (u *UserNode) AddChild(n Node) bool {
	return true
}
