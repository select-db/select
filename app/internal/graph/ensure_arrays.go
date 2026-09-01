package graph

// The bindings type every child slice on these nodes as an array —
// `children: (DBInstanceItemNode | null)[]`, not `children?: …` — and the
// frontend reads them without guarding, because the type says it is safe.
//
// Go disagrees on one point: a nil slice marshals as `null`. Nodes collect
// their children from query results, and a query that matched nothing leaves
// the slice nil, so a schema with no functions reaches the frontend as
// `"children": null` and the first `for (… of node.children)` throws.
//
// Nodes are built in a few dozen places, so rather than rely on each of them
// remembering, they are swept where they enter the graph: the whole tree after
// a build, and the touched node after a mutation — a node's children were
// already swept when they themselves entered, so a mutation has no reason to
// walk the tree again. Nullable single values — a workspace with no user, a
// node with no metadata — keep their nulls, which mean something.

func ensureArrays(workspace *WorkspaceNode) {
	if workspace == nil {
		return
	}

	if workspace.Folders == nil {
		workspace.Folders = []*FolderNode{}
	}
	if workspace.DBInstances == nil {
		workspace.DBInstances = []*DBInstanceNode{}
	}

	for _, folder := range workspace.Folders {
		ensureFolderArrays(folder)
	}
	for _, dbInstance := range workspace.DBInstances {
		ensureDBInstanceArrays(dbInstance)
	}
}

// ensureNodeArrays sweeps a single node that just entered the graph, plus the
// schema items an inserted db instance carries with it.
func ensureNodeArrays(n Node) {
	switch node := n.(type) {
	case *FolderNode:
		ensureFolderOwnArrays(node)
	case *FileNode:
		ensureFileArrays(node)
	case *DBInstanceNode:
		ensureDBInstanceOwnArrays(node)
		for _, item := range node.Children {
			ensureItemArrays(item)
		}
	case *DBInstanceItemNode:
		ensureItemArrays(node)
	}
}

func ensureFolderOwnArrays(folder *FolderNode) {
	if folder == nil {
		return
	}

	if folder.Files == nil {
		folder.Files = []*FileNode{}
	}
	if folder.Folders == nil {
		folder.Folders = []*FolderNode{}
	}
	if folder.DBInstances == nil {
		folder.DBInstances = []*DBInstanceNode{}
	}
	if folder.Badges == nil {
		folder.Badges = []string{}
	}
}

func ensureFolderArrays(folder *FolderNode) {
	if folder == nil {
		return
	}

	ensureFolderOwnArrays(folder)

	for _, file := range folder.Files {
		ensureFileArrays(file)
	}
	for _, child := range folder.Folders {
		ensureFolderArrays(child)
	}
	for _, dbInstance := range folder.DBInstances {
		ensureDBInstanceArrays(dbInstance)
	}
}

func ensureDBInstanceOwnArrays(dbInstance *DBInstanceNode) {
	if dbInstance == nil {
		return
	}

	if dbInstance.Children == nil {
		dbInstance.Children = []*DBInstanceItemNode{}
	}
	if dbInstance.Files == nil {
		dbInstance.Files = []*FileNode{}
	}
	if dbInstance.Folders == nil {
		dbInstance.Folders = []*FolderNode{}
	}
}

func ensureDBInstanceArrays(dbInstance *DBInstanceNode) {
	if dbInstance == nil {
		return
	}

	ensureDBInstanceOwnArrays(dbInstance)

	for _, item := range dbInstance.Children {
		ensureItemArrays(item)
	}
	for _, file := range dbInstance.Files {
		ensureFileArrays(file)
	}
	for _, folder := range dbInstance.Folders {
		ensureFolderArrays(folder)
	}
}

func ensureItemArrays(item *DBInstanceItemNode) {
	if item == nil {
		return
	}

	if item.Children == nil {
		item.Children = []*DBInstanceItemNode{}
	}
	if item.Badges == nil {
		item.Badges = []string{}
	}

	for _, child := range item.Children {
		ensureItemArrays(child)
	}
}

func ensureFileArrays(file *FileNode) {
	if file == nil {
		return
	}

	if file.Badges == nil {
		file.Badges = []string{}
	}
}
