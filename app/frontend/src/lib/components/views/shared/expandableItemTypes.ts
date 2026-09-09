export const expandableItemTypes = new Set([
	'folder',
	'db_instance',
	'indexes',
	'triggers',
	'tables',
	'table',
	'views',
	'schema',
	'system_catalog',
	'view',
	'columns',
	'types',
	'functions',
	'db_settings'
]);

type TreeNode = { type: string; children?: unknown };

/**
 * Whether the node carries children the tree could show under it. Read
 * defensively: a Go nil slice marshals as null, so `children` is not always the
 * array the generated type promises.
 */
export const hasChildren = (item: TreeNode): boolean =>
	Array.isArray(item.children) && item.children.length > 0;

/**
 * A node a click has nothing to open: its type never expands, and it holds no
 * children either. The second half is what keeps an index out -- "index" is not
 * an expandable type, yet it carries its columns.
 */
export const isLeafItem = (item: TreeNode): boolean =>
	!expandableItemTypes.has(item.type) && !hasChildren(item);
