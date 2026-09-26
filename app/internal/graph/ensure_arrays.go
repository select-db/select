package graph

// The bindings type every child slice on these nodes as an array,
// `children: (DatasourceItemNode | null)[]`, not `children?: ...`, and the
// frontend reads them without guarding, because the type says it is safe.
//
// Go disagrees on one point: a nil slice marshals as `null`. Nodes collect
// their children from query results, and a query that matched nothing leaves
// the slice nil, so a schema with no functions reaches the frontend as
// `"children": null` and the first `for (… of node.children)` throws.
//
// Nodes are built in a few dozen places, so rather than rely on each of them
// remembering, they are swept where they enter the graph: the whole tree after
// a build, the touched node after a mutation. A node's children were swept when
// they themselves entered, so a mutation never walks the tree. Nullable single
// values — a workspace with no user, a node with no metadata — keep their
// nulls, which mean something.

func ensureArrays(workspace *WorkspaceNode) {
	if workspace == nil {
		return
	}

	if workspace.Folders == nil {
		workspace.Folders = []*FolderNode{}
	}
	if workspace.Datasources == nil {
		workspace.Datasources = []*DatasourceNode{}
	}

	for _, folder := range workspace.Folders {
		ensureFolderArrays(folder)
	}
	for _, datasource := range workspace.Datasources {
		ensureDatasourceArrays(datasource)
	}
}

// ensureNodeArrays sweeps a single node that just entered the graph, plus the
// schema items an inserted datasource carries with it.
func ensureNodeArrays(n Node) {
	switch node := n.(type) {
	case *FolderNode:
		ensureFolderOwnArrays(node)
	case *FileNode:
		ensureFileArrays(node)
	case *DatasourceNode:
		ensureDatasourceOwnArrays(node)
		for _, item := range node.Children {
			ensureItemArrays(item)
		}
	case *DatasourceItemNode:
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
	if folder.Datasources == nil {
		folder.Datasources = []*DatasourceNode{}
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
	for _, datasource := range folder.Datasources {
		ensureDatasourceArrays(datasource)
	}
}

func ensureDatasourceOwnArrays(datasource *DatasourceNode) {
	if datasource == nil {
		return
	}

	if datasource.Children == nil {
		datasource.Children = []*DatasourceItemNode{}
	}
	if datasource.Files == nil {
		datasource.Files = []*FileNode{}
	}
	if datasource.Folders == nil {
		datasource.Folders = []*FolderNode{}
	}
}

func ensureDatasourceArrays(datasource *DatasourceNode) {
	if datasource == nil {
		return
	}

	ensureDatasourceOwnArrays(datasource)

	for _, item := range datasource.Children {
		ensureItemArrays(item)
	}
	for _, file := range datasource.Files {
		ensureFileArrays(file)
	}
	for _, folder := range datasource.Folders {
		ensureFolderArrays(folder)
	}
}

func ensureItemArrays(item *DatasourceItemNode) {
	if item == nil {
		return
	}

	if item.Children == nil {
		item.Children = []*DatasourceItemNode{}
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
