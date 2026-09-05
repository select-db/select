package graph

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/selectDb/toolkit"
)

type TestNode struct {
	ID        string
	Children  []Node
	ParentIDs []string
	Removed   map[string]bool
}

func (n *TestNode) GetIDs() []string       { return []string{n.ID} }
func (n *TestNode) GetChildren() []Node    { return n.Children }
func (n *TestNode) GetParentIDs() []string { return n.ParentIDs }
func (n *TestNode) GetPath() string        { return "" }
func (n *TestNode) AddChild(child Node) bool {
	n.Children = append(n.Children, child)
	return true
}
func (n *TestNode) RemoveChildByIDs(IDs []string) bool {
	if n.Removed == nil {
		n.Removed = make(map[string]bool)
	}
	for i, c := range n.Children {
		if toolkit.Intersects(c.GetIDs(), IDs) {
			n.Children = append(n.Children[:i], n.Children[i+1:]...)
			for _, id := range IDs {
				n.Removed[id] = true
			}
			return true
		}
	}
	return false
}

func TestAssignNonZero(t *testing.T) {
	type TestStruct struct {
		Name string
		Age  int
		Flag bool
		Ptr  *string
	}

	t.Run("assign all non-zero values", func(t *testing.T) {
		val := "hello"
		target := &TestStruct{Name: "old", Age: 0, Flag: false, Ptr: nil}
		src := TestStruct{Name: "new", Age: 10, Flag: true, Ptr: &val}

		assignNonZero(target, src)

		want := &TestStruct{Name: "new", Age: 10, Flag: true, Ptr: &val}
		if diff := cmp.Diff(target, want); diff != "" {
			t.Errorf("assignNonZero mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("skip zero values", func(t *testing.T) {
		val := "keep"
		target := &TestStruct{Name: "keep", Age: 5, Flag: true, Ptr: &val}
		src := TestStruct{Name: "", Age: 0, Flag: false, Ptr: nil}

		assignNonZero(target, src)

		want := &TestStruct{Name: "keep", Age: 5, Flag: true, Ptr: &val}
		if diff := cmp.Diff(target, want); diff != "" {
			t.Errorf("assignNonZero should skip zero values (-got +want):\n%s", diff)
		}
	})

	t.Run("partial update with some zero fields", func(t *testing.T) {
		val := "newval"
		targetVal := "oldval"
		target := &TestStruct{Name: "old", Age: 0, Flag: true, Ptr: &targetVal}
		src := TestStruct{Name: "", Age: 42, Flag: false, Ptr: &val}

		assignNonZero(target, src)

		want := &TestStruct{Name: "old", Age: 42, Flag: true, Ptr: &val}
		if diff := cmp.Diff(target, want); diff != "" {
			t.Errorf("assignNonZero partial update mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestAssignNonZero_Partial(t *testing.T) {
	type TestStruct struct {
		Name string
		Age  int
	}

	t.Run("only non-zero fields are updated", func(t *testing.T) {
		target := &TestStruct{Name: "old", Age: 5}
		src := TestStruct{Name: "", Age: 10} // only Age is non-zero

		assignNonZero(target, src)

		want := &TestStruct{Name: "old", Age: 10}
		if diff := cmp.Diff(target, want); diff != "" {
			t.Errorf("assignNonZero partial mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("all zero fields are ignored", func(t *testing.T) {
		target := &TestStruct{Name: "old", Age: 5}
		src := TestStruct{Name: "", Age: 0} // all zero

		assignNonZero(target, src)

		want := &TestStruct{Name: "old", Age: 5}
		if diff := cmp.Diff(target, want); diff != "" {
			t.Errorf("assignNonZero ignored zero fields mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("all non-zero fields overwrite", func(t *testing.T) {
		target := &TestStruct{Name: "old", Age: 5}
		src := TestStruct{Name: "new", Age: 10} // both non-zero

		assignNonZero(target, src)

		want := &TestStruct{Name: "new", Age: 10}
		if diff := cmp.Diff(target, want); diff != "" {
			t.Errorf("assignNonZero full update mismatch (-got +want):\n%s", diff)
		}
	})
}
