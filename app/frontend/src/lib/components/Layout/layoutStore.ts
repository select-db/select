import type * as graph from '$lib/wails/graph';
import { derived, get, writable } from 'svelte/store';
import { closeTab } from '$lib/components/views/shared/assistant/all';
import { clearItemSelection, setItemSelection } from '$lib/components/views/shared/sharedStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { recentItemsStore } from '$lib/stores/recentItemsStore';
import { settingsSectionLabels } from '$lib/components/views/Settings/sections';

/** Chat context DB item
 * same shape as get_chat_context databases. */
export type ChatContextDatabase = {
	id: string;
	name: string;
	uri: string;
	dialect: string;
};

/** Chat context file item. */
export type ChatContextFile = {
	id: string;
	uri: string;
	folderId?: string;
};

/**
 * TableEdit represents a single cell edit
 */
export type TableEdit = {
	databaseId: string;
	schema: string;
	table: string;
	column: string;
	value: string; // New value as string
	rowIndex: number; // Original row index in query result
	columnIndex: number; // Column index in query result
	primaryKeyValues: Record<string, unknown>; // PK column name -> value for WHERE clause
};

export type GraphConfig = {
	chartType: 'bar' | 'line' | 'area';
	xColumn: string | null;
	yColumns: string[];
	showLegend: boolean;
	stacked?: boolean;
	seriesColumn?: string | null; // when set, distinct values of this column become series
};

export type FileDatabaseTableState = {
	scrollTop?: number;
	scrollLeft?: number;
	pinnedColumns?: Set<number>;
	columnWidths?: number[];
	edits?: Record<string, TableEdit>; // Key: "rowIndex:columnIndex"
	lastResultId?: string | null;
	graphConfig?: GraphConfig;
	graphPanelCollapsed?: boolean;
	/**
	 * Per FK column, which target-table columns the user picked for display+search
	 * in the FK picker modal. Key: source column name (we're already scoped to one
	 * source table). Value: list of column names in the target table.
	 */
	foreignKeyDisplayColumns?: Record<string, string[]>;
	// NB: the full ForExport dataset is NOT stored here, GraphView re-fetches
	// it on mount (same pattern as ResultsTable / ExplainTable). Persisting it
	// would bloat localStorage with megabytes of row data.
};

/**
 * Tab - Individual tab instance with unique UUID
 */
export type Tab = {
	id: string;
	uri: string;
	file?: {
		node: graph.FileNode;
		isTemp?: boolean; // True for temporary files not persisted to disk
		content?: string; // In-memory content for temp files
		runOnOpen?: boolean; // Run the content once, as soon as the view mounts; cleared on run

		editor?: { viewState?: unknown; lintMarkers?: unknown };

		viewMode?: 'results' | 'plan' | 'explain' | 'graph';
		activeDatabaseId?: string | null;
		tableHeight?: number;
		tables?: Record<string, FileDatabaseTableState>;
		runtimeVars?: Record<string, string>; // user-supplied values for unresolved $variables
		runtimeVarTypes?: Record<string, string>; // input types per variable (ui only)
	};
	database?: { node: graph.DBInstanceNode };
	schema?: {
		databaseId?: string;
		databaseName?: string;
		selectedSchemaTable?: string;
		editor?: { viewState?: unknown };
	};
	diff?: {
		tabId: string; // for Monaco model identity
		language: string;
		panels: Array<{
			content: string;
			editor: { viewState?: unknown };
			meta?: Record<string, unknown>;
		}>;
		readOnly?: boolean;
		viewMode?: 'inline' | 'side-by-side';
		context?: 'git' | 'schema' | 'agent' | null;
		meta?: Record<string, unknown>;
	};
	terminal?: {
		sessionId: string;
		shell: string;
		outputChunks?: string[];
		exited?: boolean;
	};
	chat?: {
		sessionId: string;
		inputValue?: string;
		selectedModel?: string;
		messages?: Array<{
			id?: string;
			role: string;
			parts?: unknown[];
		}>;
		databases?: ChatContextDatabase[];
		files?: ChatContextFile[];
		scrollTop?: number;
		/** When set, task context (e.g. JSON in <task>) is injected into
		 *  system message on mount and first user message is empty; then cleared. */
		task?: string;
	};
	settings?: {
		section?: string;
		roles?: {
			selectedRoleId?: string | null;
			selectedRoleName?: string | null;
			expandedKeys?: string[];
			scrollTop?: number;
			search?: string;
		};
		// Monaco view state (cursor/scroll/selection/folding) for the personal
		// .theme / .config editors. Their tabs are synthetic and detached from any
		// layout group, so they can't ride the regular tab.file.editor path and
		// persist here instead. Restored on re-render (e.g. after Apply/Reset).
		editors?: {
			theme?: { viewState?: unknown };
			config?: { viewState?: unknown };
		};
	};
};

/**
 * TabGroup - Collection of tabs (like an editor pane)
 */
export type TabGroup = {
	id: string;
	tabs: Tab[];
	activeTabId: string | null;
};

/**
 * SplitContainer - Nested splits
 */
export type SplitContainer = {
	type: 'split';
	id: string;
	orientation: 'horizontal' | 'vertical';
	children: (TabGroup | SplitContainer)[];
	sizes: number[]; // Ratios summing to 1
};

/** Per-group tab navigation history (back/forward). */
export type TabNavigationHistory = {
	history: string[];
	index: number;
};

/**
 * Layout - Root structure
 */
export type Layout = {
	root: TabGroup | SplitContainer;
	activeGroupId: string;
	/** groupId -> visit-order history and current index */
	tabNavigationHistory: Record<string, TabNavigationHistory>;
};

const MAX_TAB_HISTORY = 50;

// Initial state: empty group
export const createInitialLayout = (): Layout => {
	const groupId = crypto.randomUUID();
	return {
		root: { id: groupId, tabs: [], activeTabId: null },
		activeGroupId: groupId,
		tabNavigationHistory: {}
	};
};

export const layoutStore = writable<Layout>(createInitialLayout());

// Helper: Traverse tree and collect all groups
const traverse = (node: TabGroup | SplitContainer, fn: (g: TabGroup) => void) => {
	if ('type' in node) {
		node.children.forEach((c) => traverse(c, fn));
	} else {
		fn(node);
	}
};

// Helper: Get all groups from a node
const collectGroups = (node: TabGroup | SplitContainer): TabGroup[] => {
	const groups: TabGroup[] = [];
	traverse(node, (g) => groups.push(g));
	return groups;
};

export const getAllGroups = (): TabGroup[] => collectGroups(get(layoutStore).root);

/**
 * The path a tab is pointing at right now.
 *
 * A tab id outlives a rename while its uri does not, so anything that acts on a
 * file later -- a debounced write, most of all -- asks for the path when it acts
 * rather than remembering the one it started with.
 */
export const getTabUri = (tabId: string): string | undefined =>
	getAllGroups()
		.flatMap((group) => group.tabs)
		.find((tab) => tab.id === tabId)?.uri;

export const activeGroupStore = derived(
	layoutStore,
	($layout) => collectGroups($layout.root).find((g) => g.id === $layout.activeGroupId) ?? null
);

export const getActiveGroup = (): TabGroup | null =>
	getAllGroups().find((g) => g.id === get(layoutStore).activeGroupId) ?? null;

export const getActiveTab = (): Tab | null => {
	const group = getActiveGroup();
	return group?.tabs.find((t) => t.id === group.activeTabId) ?? null;
};

export const findTabInGroup = (uri: string, groupId: string): Tab | null =>
	getAllGroups()
		.find((g) => g.id === groupId)
		?.tabs.find((t) => t.uri === uri) ?? null;

export const getTabByNodeId = (nodeId: string): Tab | null =>
	getAllGroups()
		.flatMap((g) => g.tabs)
		.find((t) => t.file?.node.id === nodeId || t.database?.node.id === nodeId) ?? null;

// Helper: Map over tree nodes
const mapTree = <T extends TabGroup | SplitContainer>(
	node: T,
	fn: (n: TabGroup | SplitContainer) => TabGroup | SplitContainer
): T => {
	const result = fn(node);
	if ('type' in result) {
		return { ...result, children: result.children.map((c) => mapTree(c, fn)) } as T;
	}
	return result as T;
};

// Helper: Apply an update to a specific group by id
const updateGroupById = (
	root: TabGroup | SplitContainer,
	groupId: string,
	updater: (group: TabGroup) => TabGroup
): TabGroup | SplitContainer =>
	mapTree(root, (node) =>
		'type' in node || node.id !== groupId ? node : updater(node as TabGroup)
	);

// Helper: Insert a tab immediately after the active tab in a group
const insertTabNextToActive = (group: TabGroup, tab: Tab): TabGroup => {
	const activeIndex = group.tabs.findIndex((t) => t.id === group.activeTabId);
	const insertAt = activeIndex >= 0 ? activeIndex + 1 : group.tabs.length;
	const newTabs = [...group.tabs];
	newTabs.splice(insertAt, 0, tab);

	return {
		...group,
		tabs: newTabs,
		activeTabId: tab.id
	};
};

// Helper: Wrap a target group in a split with a new group
const wrapGroupInSplit = (
	root: TabGroup | SplitContainer,
	options: {
		targetGroupId: string;
		orientation: SplitContainer['orientation'];
		newGroup: TabGroup;
		newGroupFirst?: boolean;
		duplicateExistingGroupTabs?: boolean;
	}
): TabGroup | SplitContainer => {
	const {
		targetGroupId,
		orientation,
		newGroup,
		newGroupFirst = false,
		duplicateExistingGroupTabs = true
	} = options;

	const createSplit = (node: TabGroup | SplitContainer): TabGroup | SplitContainer => {
		if ('type' in node) {
			return { ...node, children: node.children.map(createSplit) };
		}

		if (node.id !== targetGroupId) return node;

		const existingGroup = duplicateExistingGroupTabs ? { ...node, tabs: [...node.tabs] } : node;

		const children = newGroupFirst ? [newGroup, existingGroup] : [existingGroup, newGroup];

		return {
			type: 'split',
			id: crypto.randomUUID(),
			orientation,
			children,
			sizes: [0.5, 0.5]
		} as SplitContainer;
	};

	return createSplit(root);
};

/** Updates history for a group when active tab changes (always append). */
const updateTabNavigationHistory = (
	layout: Layout,
	groupId: string,
	tabId: string
): Record<string, TabNavigationHistory> => {
	const navHistory = layout.tabNavigationHistory ?? {};
	const prev = navHistory[groupId] ?? { history: [], index: -1 };
	const nextHistory = [...prev.history];
	// Truncate forward entries when navigating from a non-tip position
	if (prev.index < nextHistory.length - 1) {
		nextHistory.splice(prev.index + 1);
	}
	if (nextHistory[nextHistory.length - 1] === tabId) {
		return navHistory;
	}
	nextHistory.push(tabId);
	if (nextHistory.length > MAX_TAB_HISTORY) {
		nextHistory.shift();
	}
	return { ...navHistory, [groupId]: { history: nextHistory, index: nextHistory.length - 1 } };
};

export const setActiveTab = (groupId: string, tabId: string) => {
	layoutStore.update((layout) => {
		const newRoot = updateGroupById(layout.root, groupId, (group) => ({
			...group,
			activeTabId: tabId
		}));
		const tabNavigationHistory = updateTabNavigationHistory(layout, groupId, tabId);
		return {
			root: newRoot,
			activeGroupId: groupId,
			tabNavigationHistory
		};
	});

	syncSelectionAndRecentForActiveTab();
};

function syncSelectionAndRecentForActiveTab() {
	const tab = getActiveTab();
	const nodeId = tab?.file?.node?.id ?? tab?.database?.node?.id;
	// Tabs with no file-system item behind them (settings, terminal, chat, ...)
	// clear the selection: leaving the previous file highlighted points at
	// something the workbench is no longer showing.
	if (nodeId) setItemSelection([nodeId]);
	else clearItemSelection();
	if (tab?.file?.node) {
		const node = tab.file.node;
		recentItemsStore.addItem({
			id: node.id,
			uri: node.uri,
			type: tab.file.isTemp ? 'temp_file' : 'file',
			name: node.name,
			folderId: node.folder_id
		});
	} else if (tab?.database?.node) {
		const node = tab.database.node;
		recentItemsStore.addItem({
			id: node.id,
			uri: node.uri,
			type: 'db_instance',
			name: node.name
		});
	} else if (tab) {
		const type = tab.settings
			? 'settings'
			: tab.schema
				? 'schema'
				: tab.chat
					? 'chat'
					: tab.terminal
						? 'terminal'
						: tab.diff
							? 'diff'
							: null;
		if (type) {
			recentItemsStore.addItem({
				id: tab.id,
				uri: tab.uri,
				type,
				name: getTabLabel(tab)
			});
		}
	}
}

export function canNavigateToPreviousTab(groupId: string): boolean {
	const layout = get(layoutStore);
	const navHistory = layout.tabNavigationHistory ?? {};
	const h = navHistory[groupId];
	return !!h && h.history.length > 0 && h.index > 0;
}

export function canNavigateToNextTab(groupId: string): boolean {
	const layout = get(layoutStore);
	const navHistory = layout.tabNavigationHistory ?? {};
	const h = navHistory[groupId];
	return !!h && h.index >= 0 && h.index < h.history.length - 1;
}

export function navigateToPreviousTab(groupId: string): void {
	layoutStore.update((layout) => {
		const navHistory = layout.tabNavigationHistory ?? {};
		const h = navHistory[groupId];
		if (!h || h.index <= 0) return layout;
		const newIndex = h.index - 1;
		const tabId = h.history[newIndex];
		const group = collectGroups(layout.root).find((g) => g.id === groupId);
		if (!group || !group.tabs.some((t) => t.id === tabId)) return layout;
		return {
			...layout,
			root: updateGroupById(layout.root, groupId, (g) => ({ ...g, activeTabId: tabId })),
			activeGroupId: groupId,
			tabNavigationHistory: {
				...navHistory,
				[groupId]: { history: h.history, index: newIndex }
			}
		};
	});
	syncSelectionAndRecentForActiveTab();
}

export function navigateToNextTab(groupId: string): void {
	layoutStore.update((layout) => {
		const navHistory = layout.tabNavigationHistory ?? {};
		const h = navHistory[groupId];
		if (!h || h.index < 0 || h.index >= h.history.length - 1) return layout;
		const newIndex = h.index + 1;
		const tabId = h.history[newIndex];
		const group = collectGroups(layout.root).find((g) => g.id === groupId);
		if (!group || !group.tabs.some((t) => t.id === tabId)) return layout;
		return {
			...layout,
			root: updateGroupById(layout.root, groupId, (g) => ({ ...g, activeTabId: tabId })),
			activeGroupId: groupId,
			tabNavigationHistory: {
				...navHistory,
				[groupId]: { history: h.history, index: newIndex }
			}
		};
	});
	syncSelectionAndRecentForActiveTab();
}

// focusTab activates an existing tab by URI across all groups. Returns true if found.
export function getTabLabel(tab: Tab): string {
	if (tab.file) return tab.file.node.name;
	if (tab.database) return tab.database.node.name;
	if (tab.schema) {
		const name = tab.schema.databaseName ?? 'Schema';
		const table = tab.schema.selectedSchemaTable?.replace(':', '.') ?? '';
		return table ? `${name} · ${table}` : name;
	}
	if (tab.diff) return decodeURIComponent(tab.uri.replace(/^selectdb:\/\/diff\//, '')) || 'Diff';
	if (tab.terminal) return tab.terminal.shell?.split('/').pop() || 'Terminal';
	if (tab.chat) return 'Chat';
	if (tab.settings) {
		const section = tab.settings.section ? settingsSectionLabels[tab.settings.section] : null;
		const role = tab.settings.roles?.selectedRoleName;
		if (section && role) return `Settings · ${section} · ${role}`;
		if (section) return `Settings · ${section}`;
		return 'Settings';
	}
	return 'Tab';
}

export const focusTab = (uri: string): boolean => {
	for (const group of getAllGroups()) {
		const tab = group.tabs.find((t) => t.uri === uri);
		if (tab) {
			setActiveTab(group.id, tab.id);
			return true;
		}
	}
	return false;
};

export const addTab = (node: graph.FileNode | graph.DBInstanceNode) => {
	const group = getActiveGroup();
	if (!group) return;

	const existing = findTabInGroup(node.uri, group.id);
	if (existing) {
		setActiveTab(group.id, existing.id);
		return;
	}

	const newTab: Tab = {
		id: crypto.randomUUID(),
		uri: node.uri,
		...(node.type === 'file'
			? { file: { node: node as graph.FileNode } }
			: node.type === 'db_instance'
				? { database: { node: node as graph.DBInstanceNode } }
				: {})
	};

	layoutStore.update((layout) => ({
		...layout,
		root: updateGroupById(layout.root, group.id, (groupForUpdate) =>
			insertTabNextToActive(groupForUpdate, newTab)
		)
	}));

	if (node.type === 'file') {
		recentItemsStore.addItem({
			id: node.id,
			uri: node.uri,
			type: 'file',
			name: node.name,
			folderId: (node as graph.FileNode).folder_id
		});
	} else if (node.type === 'db_instance') {
		recentItemsStore.addItem({
			id: node.id,
			uri: node.uri,
			type: 'db_instance',
			name: node.name
		});
	}
};

export const addSchemaTab = (databaseId?: string, databaseName?: string) => {
	const group = getActiveGroup();
	if (!group) return;

	const tabId = crypto.randomUUID();
	const newTab: Tab = {
		id: tabId,
		uri: `selectdb://schema/${tabId}`,
		schema: { databaseId, databaseName }
	};

	layoutStore.update((layout) => ({
		...layout,
		root: updateGroupById(layout.root, group.id, (groupForUpdate) =>
			insertTabNextToActive(groupForUpdate, newTab)
		)
	}));
	syncSelectionAndRecentForActiveTab();
};

export const updateSettingsTab = (patch: Exclude<Tab['settings'], undefined>) => {
	layoutStore.update((layout) => ({
		...layout,
		root: mapTree(layout.root, (n) => {
			if ('type' in n) return n;
			return {
				...n,
				tabs: n.tabs.map((t) =>
					t.settings !== undefined ? { ...t, settings: { ...t.settings, ...patch } } : t
				)
			};
		})
	}));
};

/**
 * Opens the Settings tab at an optional section (theme/config/workspace/...).
 * Reuses an already-open Settings tab instead of duplicating it, and jumps to
 * the requested section (Settings reacts to the tab's `settings.section`).
 */
export const openSettingsSection = (section?: string) => {
	if (focusTab('selectdb://settings')) {
		if (section) updateSettingsTab({ section });
		return;
	}

	const group = getActiveGroup();
	if (!group) return;

	const newTab: Tab = {
		id: crypto.randomUUID(),
		uri: 'selectdb://settings',
		settings: section ? { section } : {}
	};

	layoutStore.update((layout) => ({
		...layout,
		root: updateGroupById(layout.root, group.id, (groupForUpdate) =>
			insertTabNextToActive(groupForUpdate, newTab)
		)
	}));

	syncSelectionAndRecentForActiveTab();
};

export const addTempFileTab = (
	params: {
		content: string;
		name: string;
		dbInstanceId?: string;
		folderId: string;
		/** Execute the content against the attached database as soon as the tab mounts. */
		runOnOpen?: boolean;
	},
	split: boolean = true
) => {
	const group = getActiveGroup();
	if (!group) return '';

	const newTabId = crypto.randomUUID();

	const tempUri = `temp://${newTabId}`;
	const virtualNode = {
		id: tempUri,
		uri: tempUri,
		type: 'file',
		name: params.name,
		folder_id: params.folderId,
		databases: params.dbInstanceId ? [{ id: params.dbInstanceId, name: '' }] : [],
		badges: []
	} as unknown as graph.FileNode;

	const newTab: Tab = {
		id: newTabId,
		uri: tempUri,
		file: {
			node: virtualNode,
			isTemp: true,
			content: params.content,
			...(params.runOnOpen && { runOnOpen: true })
		}
	};

	const trackRecentTempFile = () => {
		recentItemsStore.addItem({
			id: virtualNode.id,
			uri: virtualNode.uri,
			type: 'temp_file',
			name: virtualNode.name,
			folderId: virtualNode.folder_id
		});
	};

	// Add to current group without splitting
	if (!split) {
		layoutStore.update((layout) => ({
			...layout,
			root: updateGroupById(layout.root, group.id, (groupForUpdate) =>
				insertTabNextToActive(groupForUpdate, newTab)
			)
		}));
		trackRecentTempFile();
		return;
	}

	// Create a split view (default behavior)
	const newGroupId = crypto.randomUUID();

	layoutStore.update((layout) => {
		const newGroup: TabGroup = {
			id: newGroupId,
			tabs: [newTab],
			activeTabId: newTab.id
		};

		return {
			...layout,
			root: wrapGroupInSplit(layout.root, {
				targetGroupId: group.id,
				orientation: 'vertical',
				newGroup,
				newGroupFirst: false,
				duplicateExistingGroupTabs: true
			}),
			activeGroupId: newGroupId
		};
	});

	trackRecentTempFile();
};

export type AddDiffTabParams = {
	panels: Array<{
		content: string;
		meta?: Record<string, unknown>;
	}>;
	language: string;
	readOnly?: boolean;
	viewMode?: 'inline' | 'side-by-side';
	context?: 'git' | 'schema' | 'agent' | null;
	meta?: Record<string, unknown>;
	title?: string;
	targetGroupId?: string;
	split?: boolean;
};

export const addDiffTab = (params: AddDiffTabParams) => {
	const group = getActiveGroup();
	if (!group) return;

	const tabId = crypto.randomUUID();
	const diffTabId = `diff://${tabId}`;
	const uri = params.title
		? `selectdb://diff/${encodeURIComponent(params.title)}`
		: `selectdb://diff/${tabId}`;

	const panels = params.panels.map((p) => ({
		content: p.content,
		editor: {} as { viewState?: unknown },
		meta: p.meta
	}));

	const newTab: Tab = {
		id: tabId,
		uri,
		diff: {
			tabId: diffTabId,
			language: params.language,
			panels,
			readOnly: params.readOnly ?? true,
			viewMode: params.viewMode ?? 'side-by-side',
			context: params.context ?? null,
			meta: params.meta
		}
	};

	if (params.targetGroupId) {
		layoutStore.update((layout) => ({
			...layout,
			root: updateGroupById(layout.root, params.targetGroupId!, (groupForUpdate) =>
				insertTabNextToActive(groupForUpdate, newTab)
			)
		}));
		return;
	}

	if (!params.split) {
		layoutStore.update((layout) => ({
			...layout,
			root: updateGroupById(layout.root, group.id, (groupForUpdate) =>
				insertTabNextToActive(groupForUpdate, newTab)
			)
		}));
		return;
	}

	const newGroupId = crypto.randomUUID();
	layoutStore.update((layout) => {
		const newGroup: TabGroup = {
			id: newGroupId,
			tabs: [newTab],
			activeTabId: newTab.id
		};

		return {
			...layout,
			root: wrapGroupInSplit(layout.root, {
				targetGroupId: group.id,
				orientation: 'vertical',
				newGroup,
				newGroupFirst: false,
				duplicateExistingGroupTabs: true
			}),
			activeGroupId: newGroupId
		};
	});
};

export const addTerminalTab = (shell: string = '') => {
	const group = getActiveGroup();
	if (!group) return;

	const tabId = crypto.randomUUID();

	const newTab: Tab = {
		id: tabId,
		uri: `selectdb://terminal/${tabId}`,
		terminal: { sessionId: tabId, shell }
	};

	const activeTab = getActiveTab();
	if (activeTab?.terminal) {
		layoutStore.update((layout) => ({
			...layout,
			root: updateGroupById(layout.root, group.id, (g) => insertTabNextToActive(g, newTab))
		}));
		syncSelectionAndRecentForActiveTab();
		return;
	}

	const newGroupId = crypto.randomUUID();
	layoutStore.update((layout) => {
		const newGroup: TabGroup = {
			id: newGroupId,
			tabs: [newTab],
			activeTabId: newTab.id
		};

		return {
			...layout,
			root: wrapGroupInSplit(layout.root, {
				targetGroupId: group.id,
				orientation: 'horizontal',
				newGroup,
				newGroupFirst: false,
				duplicateExistingGroupTabs: false
			}),
			activeGroupId: newGroupId
		};
	});
	syncSelectionAndRecentForActiveTab();
};

export const dbInstanceToContextDb = (db: graph.DBInstanceNode): ChatContextDatabase => ({
	id: db.id ?? '',
	name: db.name ?? '',
	uri: db.uri ?? '',
	dialect: db.db_type ?? ''
});

/** Opens chat in a new pane. Optional task: object is JSON-serialized into <task> and injected into system message; first user message is empty (not shown). */
export const addChatTab = (task?: Record<string, unknown>) => {
	const group = getActiveGroup();
	const ws = get(workspaceGraphStore);
	if (!group || !ws) return;

	const activeTab = getActiveTab();
	const allDbs = ws.db_instances.map(dbInstanceToContextDb);

	let databases: ChatContextDatabase[] = allDbs;
	let files: ChatContextFile[] = [];

	const taskDatabaseId = task && typeof task.databaseId === 'string' ? task.databaseId : undefined;

	if (activeTab?.file?.node) {
		const file = activeTab.file.node;
		const fileDbIds = file.databases?.map((d) => d.id).filter(Boolean) ?? [];
		if (fileDbIds.length > 0) {
			databases = fileDbIds
				.map((id) => allDbs.find((d) => d.id === id))
				.filter((d): d is ChatContextDatabase => d != null);
		} else {
			const activeId = activeTab.file?.activeDatabaseId ?? taskDatabaseId ?? undefined;
			const one = activeId ? allDbs.find((d) => d.id === activeId) : undefined;
			if (one) databases = [one];
		}
		// Temp tabs: ResourceMenu / getOpenTabOptions use Tab.id; virtual FileNode.id is temp://…
		files = [
			{
				uri: activeTab.uri,
				id: activeTab.file.isTemp ? activeTab.id : file.id,
				folderId: file.folder_id
			}
		];
	} else if (activeTab?.database?.node) {
		databases = [dbInstanceToContextDb(activeTab.database.node)];
	} else if (activeTab?.schema?.databaseId) {
		const one = allDbs.find((d) => d.id === activeTab.schema!.databaseId);
		if (one) databases = [one];
	}

	const taskBlock =
		task != null &&
		Object.keys(task).length > 0 &&
		`<task>\n${JSON.stringify(task, null, 2)}\n</task>`;

	const tabId = crypto.randomUUID();
	const newTab: Tab = {
		id: tabId,
		uri: `selectdb://chat/${tabId}`,
		chat: {
			sessionId: tabId,
			databases: databases.length > 0 ? databases : undefined,
			files: files.length > 0 ? files : undefined,
			...(taskBlock && { task: taskBlock })
		}
	};

	if (activeTab?.chat) {
		layoutStore.update((layout) => ({
			...layout,
			root: updateGroupById(layout.root, group.id, (g) => insertTabNextToActive(g, newTab))
		}));
		syncSelectionAndRecentForActiveTab();
		return;
	}

	const newGroupId = crypto.randomUUID();
	layoutStore.update((layout) => {
		const newGroup: TabGroup = {
			id: newGroupId,
			tabs: [newTab],
			activeTabId: newTab.id
		};
		return {
			...layout,
			root: wrapGroupInSplit(layout.root, {
				targetGroupId: group.id,
				orientation: 'vertical',
				newGroup,
				newGroupFirst: false,
				duplicateExistingGroupTabs: true
			}),
			activeGroupId: newGroupId
		};
	});
	syncSelectionAndRecentForActiveTab();
};

// Helper: Calculate new active tab when removing a tab
const getNewActiveTab = (
	tabs: Tab[],
	removedTabId: string,
	currentActiveId: string | null
): string | null => {
	if (currentActiveId !== removedTabId) return currentActiveId;

	const idx = tabs.findIndex((t) => t.id === removedTabId);
	const newTabs = tabs.filter((t) => t.id !== removedTabId);
	return newTabs[idx === 0 ? 0 : idx - 1]?.id ?? null;
};

// Helper: Remove a tab from the layout tree
const removeTabFromTree = (
	node: TabGroup | SplitContainer,
	tabId: string,
	options: { dropEmptyGroups: boolean }
): { node: TabGroup | SplitContainer | null; removedTab: Tab | null } => {
	const { dropEmptyGroups } = options;

	if ('type' in node) {
		let removedTab: Tab | null = null;

		const children = node.children
			.map((child) => {
				const result = removeTabFromTree(child, tabId, options);
				if (result.removedTab) {
					removedTab = result.removedTab;
				}
				return result.node;
			})
			.filter((c): c is (typeof node.children)[0] => c !== null);

		if (children.length === 0) {
			return { node: null, removedTab };
		}

		if (children.length === 1) {
			return { node: children[0], removedTab };
		}

		return {
			node: {
				...node,
				children,
				sizes: children.map(() => 1 / children.length)
			},
			removedTab
		};
	}

	const tab = node.tabs.find((t) => t.id === tabId);
	if (!tab) {
		return { node, removedTab: null };
	}

	const newTabs = node.tabs.filter((t) => t.id !== tabId);

	if (dropEmptyGroups && newTabs.length === 0) {
		return { node: null, removedTab: tab };
	}

	return {
		node: {
			...node,
			tabs: newTabs,
			activeTabId: getNewActiveTab(node.tabs, tabId, node.activeTabId)
		},
		removedTab: tab
	};
};

export const removeTab = (tabId: string) => {
	const tab = getAllGroups()
		.flatMap((g) => g.tabs)
		.find((t) => t.id === tabId);

	if (tab) closeTab(tab);

	layoutStore.update((layout) => {
		const { node: newRoot } = removeTabFromTree(layout.root, tabId, { dropEmptyGroups: true });
		if (!newRoot) return createInitialLayout();

		// Remove tabId from the affected group's history and adjust index
		const groupWithTab = collectGroups(layout.root).find((g) => g.tabs.some((t) => t.id === tabId));
		let tabNavigationHistory = layout.tabNavigationHistory ?? {};
		if (groupWithTab && tabNavigationHistory[groupWithTab.id]) {
			const h = tabNavigationHistory[groupWithTab.id];
			const i = h.history.indexOf(tabId);
			if (i >= 0) {
				const history = h.history.filter((id) => id !== tabId);
				let index = h.index;
				if (i < index) index -= 1;
				else if (i === index && index >= history.length) index = Math.max(0, history.length - 1);
				index = Math.min(index, history.length - 1);
				tabNavigationHistory = {
					...tabNavigationHistory,
					[groupWithTab.id]: { history, index: index < 0 ? -1 : index }
				};
			}
		}

		return cleanupLayout({ ...layout, root: newRoot, tabNavigationHistory });
	});
};

export const removeTabByUri = (uri: string) => {
	getAllGroups()
		.flatMap((g) => g.tabs)
		.filter((t) => t.uri === uri)
		.forEach((tab) => removeTab(tab.id));
};

/** Updates open file tabs when a file is renamed so they point at the new URI (keeps tab open). */
/**
 * Points open tabs at a renamed path.
 *
 * Renaming a folder moves everything under it, so a tab is updated when it is
 * the renamed file itself or when it sits inside the renamed folder — otherwise
 * it keeps reading a path that no longer exists, and the file quietly opens a
 * second time when it is opened again.
 */
export const updateFileTabsAfterRename = (oldUri: string, newUri: string, newName: string) => {
	const insideOld = `${oldUri}/`;
	const tabsToUpdate = getAllGroups()
		.flatMap((g) => g.tabs)
		.filter((t) => t.file && !t.file.isTemp && (t.uri === oldUri || t.uri.startsWith(insideOld)));

	for (const tab of tabsToUpdate) {
		const node = tab.file!.node;
		const uri = tab.uri === oldUri ? newUri : newUri + tab.uri.slice(oldUri.length);
		// Only the renamed thing takes the new name; a file carried along by a
		// folder rename keeps its own.
		const name = tab.uri === oldUri ? newName : node.name;

		updateTab({
			...tab,
			uri,
			file: {
				...tab.file!,
				node: { ...node, id: uri, uri, name } as graph.FileNode
			}
		});
	}
};

export const updateTab = (tab: Tab) => {
	layoutStore.update((layout) => ({
		...layout,
		root: mapTree(layout.root, (n) =>
			'type' in n ? n : { ...n, tabs: n.tabs.map((t) => (t.id === tab.id ? tab : t)) }
		)
	}));
};

export const reorderTabs = (groupId: string, fromIndex: number, toIndex: number) => {
	layoutStore.update((layout) => ({
		...layout,
		root: updateGroupById(layout.root, groupId, (group) => {
			const newTabs = [...group.tabs];
			const [moved] = newTabs.splice(fromIndex, 1);
			newTabs.splice(toIndex, 0, moved);
			return { ...group, tabs: newTabs };
		})
	}));
};

export const moveTabToGroup = (tabId: string, targetGroupId: string, insertIndex?: number) => {
	layoutStore.update((layout) => {
		const { node: rootAfterRemove, removedTab } = removeTabFromTree(layout.root, tabId, {
			dropEmptyGroups: true
		});

		if (!rootAfterRemove || !removedTab) return layout;

		const rootAfterAdd = updateGroupById(rootAfterRemove, targetGroupId, (group) => {
			const newTabs = [...group.tabs];
			newTabs.splice(insertIndex ?? newTabs.length, 0, removedTab);

			return { ...group, tabs: newTabs, activeTabId: removedTab.id };
		});

		return cleanupLayout({
			...layout,
			root: rootAfterAdd,
			activeGroupId: targetGroupId
		});
	});
};

export const splitGroup = (
	groupId: string,
	direction: 'up' | 'down' | 'left' | 'right',
	tabId?: string
) => {
	layoutStore.update((layout) => {
		const newGroupId = crypto.randomUUID();
		const orientation = direction === 'left' || direction === 'right' ? 'vertical' : 'horizontal';
		const newGroupFirst = direction === 'up' || direction === 'left';

		let movedTab: Tab | null = null;

		// Remove tab from its current group if tabId provided
		if (tabId) {
			const result = removeTabFromTree(layout.root, tabId, { dropEmptyGroups: false });
			movedTab = result.removedTab;
			layout = { ...layout, root: result.node ?? layout.root };
		}

		const newGroup: TabGroup = {
			id: newGroupId,
			tabs: movedTab ? [movedTab] : [],
			activeTabId: movedTab?.id ?? null
		};

		const rootWithSplit = wrapGroupInSplit(layout.root, {
			targetGroupId: groupId,
			orientation,
			newGroup,
			newGroupFirst,
			duplicateExistingGroupTabs: false
		});

		return cleanupLayout({
			...layout,
			root: rootWithSplit,
			activeGroupId: newGroupId
		});
	});
};

const cleanupLayout = (layout: Layout): Layout => {
	const cleanNode = (node: TabGroup | SplitContainer): typeof node | null => {
		if ('type' in node) {
			const cleanedChildren = node.children
				.map(cleanNode)
				.filter((child): child is TabGroup | SplitContainer => child !== null);

			if (cleanedChildren.length === 0) return null;
			if (cleanedChildren.length === 1) return cleanedChildren[0];

			return {
				...node,
				children: cleanedChildren,
				sizes: cleanedChildren.map(() => 1 / cleanedChildren.length)
			};
		}

		return node.tabs.length === 0 ? null : node;
	};

	const cleanedRoot = cleanNode(layout.root);

	if (!cleanedRoot) return createInitialLayout();

	const allGroups = collectGroups(cleanedRoot);
	const activeGroupExists = allGroups.some((g) => g.id === layout.activeGroupId);
	const groupIds = new Set(allGroups.map((g) => g.id));
	const tabNavigationHistory = { ...(layout.tabNavigationHistory ?? {}) };
	for (const id of Object.keys(tabNavigationHistory)) {
		if (!groupIds.has(id)) delete tabNavigationHistory[id];
	}

	return {
		...layout,
		root: cleanedRoot,
		activeGroupId: activeGroupExists
			? layout.activeGroupId
			: (allGroups[0]?.id ?? layout.activeGroupId),
		tabNavigationHistory
	};
};
