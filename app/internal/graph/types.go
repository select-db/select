package graph

type Node interface {
	GetIDs() []string
	GetChildren() []Node
	GetParentIDs() []string
	RemoveChildByIDs(IDs []string) bool
	AddChild(child Node) bool
}
