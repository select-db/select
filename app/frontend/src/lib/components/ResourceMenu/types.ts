import type { graph } from '$lib/wailsjs/go/models';

export type ResourceType =
	| 'file'
	| 'db_instance'
	| 'db_item'
	| 'temp_file'
	| 'quick_action'
	| 'settings'
	| 'schema'
	| 'chat'
	| 'terminal'
	| 'diff';

export type ResourceNode = graph.FileNode | graph.DBInstanceNode | graph.DBInstanceItemNode;

export type ResourceMenuOption = {
	id: string;
	label: string;
	type: ResourceType;
	uri: string;
	node: ResourceNode;
	folderId?: string;
};

export type ResourceMenuAction = (option: ResourceMenuOption) => void | Promise<void>;

/** When set, only enabled DBs/schemas contribute db_instance / db_item hits; files are unaffected. */
export type ResourceSearchScope = {
	knownSchemaIds: ReadonlySet<string>;
	enabledDbIds: ReadonlySet<string>;
	enabledSchemaIds: ReadonlySet<string>;
};

export type ResourceMenuProps = {
	types: ResourceType[];
	action: ResourceMenuAction;
	multiple?: boolean;
	selectedIds?: string[];
	excludeIds?: string[];
	placeholder?: string;
	width?: number;
	maxHeight?: number;
	noBorder?: boolean;
	onClose?: () => void;
	extraOptions?: ResourceMenuOption[];
	searchScope?: ResourceSearchScope;
	/** When bound, persists the search field for the parent (e.g. workspace search modal). */
	searchQuery?: string;
};
