package graph

import (
	"reflect"

	"github.com/selectDb/toolkit"
)

func FindNodesByIds(root Node, IDs []string) []Node {
	result := []Node{}

	var dfs func(n Node)
	dfs = func(n Node) {
		if toolkit.Intersects(n.GetIDs(), IDs) {
			result = append(result, n)
		}
		for _, child := range n.GetChildren() {
			dfs(child)
		}
	}

	dfs(root)
	return result
}

func RemoveNodesByIDs(root Node, IDs []string) bool {
	targets := FindNodesByIds(root, IDs)
	if len(targets) == 0 {
		return false
	}

	removed := false
	for _, target := range targets {
		PIDs := target.GetParentIDs()
		parents := FindNodesByIds(root, PIDs)
		for _, parent := range parents {
			if parent.RemoveChildByIDs(IDs) {
				removed = true
			}
		}
	}

	return removed
}

func assignNonZero(target interface{}, src interface{}) {
	tv := reflect.ValueOf(target).Elem()
	sv := reflect.ValueOf(src)

	for i := 0; i < sv.NumField(); i++ {
		sf := sv.Field(i)
		tf := tv.FieldByName(sv.Type().Field(i).Name)

		if !tf.IsValid() || !tf.CanSet() {
			continue
		}

		switch sf.Kind() {
		case reflect.Ptr:
			if sf.IsNil() {
				continue
			}
			se := sf.Elem()
			// Handle *string -> string
			if se.Kind() == reflect.String && tf.Kind() == reflect.String {
				if se.String() != "" {
					tf.SetString(se.String())
				}
				// Handle *T -> *T (e.g. *QueryResult)
			} else if tf.Kind() == reflect.Ptr && tf.Type() == sf.Type() {
				tf.Set(sf)
				// Handle *T -> T: non-nil pointer means explicitly set, always assign
			} else if tf.Kind() == se.Kind() {
				tf.Set(se)
			}

		default:
			// Handle direct assignment (string->string, int->int, bool->bool, etc.)
			if tf.Kind() == sf.Kind() && !isZero(sf) {
				tf.Set(sf)
			}
		}
	}
}

func isZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return true
		}
		return isZero(v.Elem())
	case reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
		return v.Len() == 0
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	}

	return false
}

// GetDBInstanceNodeByID returns the DBInstanceNode with the given ID from the
// current WorkspaceGraph, or nil if no such node exists.
func (g *Graph) GetDBInstanceNodeByID(ID string) *DBInstanceNode {
	g.mu.RLock()
	root := g.WorkspaceGraph
	g.mu.RUnlock()

	if root == nil {
		return nil
	}

	for _, n := range root.DBInstances {
		if n.ID == ID || n.URI == ID {
			return n
		}
	}
	return nil
}

// GetFileNodeByID returns the FileNode with the given ID from the
// current WorkspaceGraph, or nil if no such node exists.
func (g *Graph) GetFileNodeByID(fileID string) *FileNode {
	g.mu.RLock()
	root := g.WorkspaceGraph
	g.mu.RUnlock()

	if root == nil {
		return nil
	}

	nodes := FindNodesByIds(root, []string{fileID})
	if len(nodes) == 0 {
		return nil
	}
	if fn, ok := nodes[0].(*FileNode); ok {
		return fn
	}
	return nil
}
