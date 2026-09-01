package graph

import (
	"reflect"
)

func assignNonZero(target interface{}, src interface{}) {
	tv := reflect.ValueOf(target).Elem()
	sv := reflect.ValueOf(src)

	for i := 0; i < sv.NumField(); i++ {
		sf := sv.Field(i)
		tf := tv.FieldByName(sv.Type().Field(i).Name) // nosemgrep: go.lang.security.audit.unsafe-reflect-by-name.unsafe-reflect-by-name -- field name comes from the source struct's own compile-time type, never user input

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
// current WorkspaceGraph, or nil if no such node exists. A db instance answers
// to both its config ID and its URI.
func (g *Graph) GetDBInstanceNodeByID(ID string) *DBInstanceNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, _ := g.lookup(ID).(*DBInstanceNode)
	return node
}

// GetFileNodeByID returns the FileNode with the given ID from the
// current WorkspaceGraph, or nil if no such node exists.
//
// A file whose folder has never been opened is not in the graph yet, so a miss
// resolves the folders along the file's path before giving up. That keeps
// callers — a link into a file, a tab restored from the last session — working
// without knowing which folders have been opened.
func (g *Graph) GetFileNodeByID(fileID string) *FileNode {
	node, _ := g.nodeForURI(fileID).(*FileNode)
	return node
}
