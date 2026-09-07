import type * as graph from '$lib/wails/graph';
import { get } from 'svelte/store';
import {
	setItemSelection,
	toggleItemSelection,
	toggleIsItemExpanded,
	addToItemSelection,
	setFocusedFsItem,
	requestFsPanelFocus
} from '$lib/components/views/shared/sharedStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { loadSchema } from '$lib/utils/query/loadSchema';
import { navigateToFile } from '$lib/components/views/shared/navigateToFile';
import { navigateToGitFile } from '$lib/components/views/Git/navigateToGitFile';
import { navigateToSchema } from '$lib/components/views/Schema/navigateToSchema';
import { expandableItemTypes } from '$lib/components/views/shared/expandableItemTypes';
import { navigateToMatch } from '$lib/components/views/Search/navigateToMatch';

/**
 * A click that means "select", not "open": shift for a range, and the platform's
 * own multi-select key -- cmd on macOS, ctrl everywhere else, which is why both
 * are accepted rather than metaKey alone.
 */
const isSelectionClick = (event?: MouseEvent) =>
	!!(event?.shiftKey || event?.metaKey || event?.ctrlKey);

const handleSpecialClick = (
	itemId: string,
	lastClickedId: { current: string | null },
	event?: MouseEvent
) => {
	const isShiftClick = event?.shiftKey && lastClickedId.current;
	const isCtrlClick = event?.metaKey || event?.ctrlKey;

	if (isShiftClick) {
		// Range selection
		const rangeIds = getRangeSelection(lastClickedId.current!, itemId);
		for (const id of rangeIds) addToItemSelection(id);
		// setItemSelection(rangeIds);
		return;
	}

	if (isCtrlClick) {
		// Toggle single folder selection
		toggleItemSelection(itemId);
		lastClickedId.current = itemId;
		return;
	}
};

/**
 * Create folder click handler with selection support
 */
export const createFolderClickHandler = (
	ctx: 'fs' | 'git' | 'search',
	lastClickedId: { current: string | null }
) => {
	return (folder: graph.FolderNode, event?: MouseEvent) => {
		if (ctx !== 'fs') {
			toggleIsItemExpanded(folder.id);
			return;
		}

		if (isSelectionClick(event)) {
			handleSpecialClick(folder.id, lastClickedId, event);
			return;
		}

		// Normal click behavior
		setItemSelection([folder.id]);
		toggleIsItemExpanded(folder.id);
		setFocusedFsItem(folder.id);
		requestFsPanelFocus();
		lastClickedId.current = folder.id;
		return;
	};
};

/**
 * Create file click handler with selection support
 */
export const createFileClickHandler = (
	ctx: 'fs' | 'git' | 'search',
	lastClickedId: { current: string | null }
) => {
	return async (file: graph.FileNode, event?: MouseEvent) => {
		if (ctx === 'search') {
			// Search context: navigate to match with line/column positioning
			await navigateToMatch(file);
			return;
		}

		if (ctx === 'git') {
			await navigateToGitFile(file);
			return;
		}

		if (isSelectionClick(event)) {
			handleSpecialClick(file.id, lastClickedId, event);
			return;
		}

		// Special handling for schema.sql files inside database folders
		if (file.name === 'schema.sql' && file.folder_id) {
			const workspace = get(workspaceGraphStore);
			// Find database whose URI matches the file's folder_id
			const database = (workspace?.db_instances ?? []).find((db) => db.uri === file.folder_id);
			if (database) {
				setItemSelection([file.id]);
				await navigateToSchema(database.id);
				setFocusedFsItem(file.id);
				requestFsPanelFocus();
				lastClickedId.current = file.id;
				return;
			}
		}

		// Clear selection and navigate
		await navigateToFile(file);
		setFocusedFsItem(file.id);
		requestFsPanelFocus();
		lastClickedId.current = file.id;
		return;
	};
};

const clickItem = async (item: graph.DBInstanceNode | graph.DBInstanceItemNode) => {
	toggleIsItemExpanded(item.id);

	if (!expandableItemTypes.has(item.type)) return;

	const shouldLoadSchema =
		item.type === 'db_instance' && 'children' in item && item.children.length === 0;

	if (shouldLoadSchema) loadSchema({ database: item as graph.DBInstanceNode });

	setItemSelection([item.id]);
};

/**
 * Create database click handler with selection support
 */
export const createDatabaseClickHandler = (
	ctx: 'fs' | 'git' | 'search',
	lastClickedId: { current: string | null }
) => {
	return (item: graph.DBInstanceNode | graph.DBInstanceItemNode, event?: MouseEvent) => {
		const isDbInstance = item.type === 'db_instance';

		if (ctx !== 'fs' || !isDbInstance) {
			clickItem(item);
			return;
		}

		if (isSelectionClick(event)) {
			handleSpecialClick(item.id, lastClickedId, event);
			return;
		}

		// Single click behavior
		clickItem(item);
		setFocusedFsItem(item.id);
		requestFsPanelFocus();
		lastClickedId.current = item.id;
	};
};

/**
 * Create all click handlers
 */
export const createClickHandlers = (
	ctx: 'fs' | 'git' | 'search',
	lastClickedId: { current: string | null }
) => {
	return {
		handleFolderClick: createFolderClickHandler(ctx, lastClickedId),
		handleFileClick: createFileClickHandler(ctx, lastClickedId),
		handleDatabaseClick: createDatabaseClickHandler(ctx, lastClickedId)
	};
};

type SelectableItem = graph.FileNode | graph.FolderNode | graph.DBInstanceNode;

/**
 * Find an item and its parent path in the graph
 */
const findItemWithParent = (
	id: string,
	files: graph.FileNode[],
	folders: graph.FolderNode[],
	databases: graph.DBInstanceNode[],
	parentId: string | null = null
): { item: SelectableItem; parentId: string | null } | null => {
	// Check files
	for (const file of files) {
		if (file.id === id) return { item: file, parentId };
	}

	// Check databases
	for (const db of databases) {
		if (db.id === id) return { item: db, parentId };
	}

	// Check folders
	for (const folder of folders) {
		if (folder.id === id) return { item: folder, parentId };

		// Recursively search in folder's children
		const result = findItemWithParent(
			id,
			folder.files,
			folder.folders,
			folder.db_instances,
			folder.id
		);
		if (result) return result;
	}

	return null;
};

/**
 * Get items at the same level (same parent) in display order
 */
const getItemsAtLevel = (parentFolder: graph.FolderNode): SelectableItem[] => {
	const items: SelectableItem[] = [];

	// Order: folders, databases, files (matching the display order in FileItems.svelte)
	items.push(...parentFolder.folders);
	items.push(...parentFolder.db_instances);
	items.push(...parentFolder.files);

	return items;
};

/**
 * Calculate range selection between two items
 * Only selects items if they share the same parent (same depth level)
 */
export const getRangeSelection = (fromId: string, toId: string): string[] => {
	const workspace = get(workspaceGraphStore);
	if (!workspace) return [];

	const root = workspace.folders[0];
	if (!root) return [];

	// Find both items with their parent information
	const fromResult = findItemWithParent(fromId, root.files, root.folders, root.db_instances);
	const toResult = findItemWithParent(toId, root.files, root.folders, root.db_instances);

	if (!fromResult || !toResult) return [];

	// Check if items are at the same level (same parent)
	if (fromResult.parentId !== toResult.parentId) {
		return []; // Different levels, don't select
	}

	// Get all items at this level
	let levelItems: SelectableItem[];
	if (fromResult.parentId === null) {
		// Root level
		levelItems = getItemsAtLevel(root);
	} else {
		// Find the parent folder
		const parentResult = findItemWithParent(
			fromResult.parentId,
			root.files,
			root.folders,
			root.db_instances
		);
		if (!parentResult || parentResult.item.type !== 'folder') return [];
		levelItems = getItemsAtLevel(parentResult.item as graph.FolderNode);
	}

	// Find indices in the level items
	const fromIndex = levelItems.findIndex((item) => item.id === fromId);
	const toIndex = levelItems.findIndex((item) => item.id === toId);

	if (fromIndex === -1 || toIndex === -1) return [];

	// Select range
	const start = Math.min(fromIndex, toIndex);
	const end = Math.max(fromIndex, toIndex);

	const selectedIds = [];
	for (let i = start; i <= end; i++) {
		selectedIds.push(levelItems[i].id);
	}

	return selectedIds;
};

/**
 * How long a single click is held before it is acted on, for items that also
 * answer to a double-click. Long enough to swallow a normal double-click, short
 * enough that expanding the item still feels immediate. A double-click slower
 * than this expands the item first, which is the same outcome as before.
 */
const DOUBLE_CLICK_DELAY_MS = 200;

/**
 * Separates single from double clicks on the same item.
 *
 * A double-click is preceded by two plain clicks, so without this a table
 * visibly expands and collapses again before its data opens. The single-click
 * action is held back for DOUBLE_CLICK_DELAY_MS and dropped when a double-click
 * lands first. Items that opt out of `shouldDefer` keep reacting immediately, so
 * only the ones with a double-click action pay the delay.
 *
 * `cancel` must run on destroy so a pending click cannot fire after unmount.
 */
export const createDeferredClickHandlers = <T>({
	shouldDefer,
	onClick,
	onDoubleClick
}: {
	shouldDefer: (item: T) => boolean;
	onClick: (item: T, event?: MouseEvent) => void;
	onDoubleClick: (item: T) => void;
}) => {
	let pending: ReturnType<typeof setTimeout> | null = null;

	const cancel = () => {
		if (pending === null) return;
		clearTimeout(pending);
		pending = null;
	};

	return {
		handleClick: (item: T, event?: MouseEvent) => {
			if (!shouldDefer(item)) {
				onClick(item, event);
				return;
			}

			cancel();
			pending = setTimeout(() => {
				pending = null;
				onClick(item, event);
			}, DOUBLE_CLICK_DELAY_MS);
		},
		handleDoubleClick: (item: T) => {
			cancel();
			onDoubleClick(item);
		},
		cancel
	};
};
