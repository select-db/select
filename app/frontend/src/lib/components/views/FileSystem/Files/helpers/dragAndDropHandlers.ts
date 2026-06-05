import { get } from 'svelte/store';

import type { graph } from '$lib/wailsjs/go/models';
import { Rename } from '$lib/wailsjs/go/fs_provider/FSProvider';

import { must, tryCatch } from '$lib/utils/tryCatch';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';

import { AlertType } from '$lib/system/Alert/types';
import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';

import {
	selectedItemsStore,
	setItemSelection,
	startDrag,
	setHoveredTarget,
	endDrag,
	dragStateStore,
	expandItem
} from '$lib/components/views/shared/sharedStore';

import { updateFileTabsAfterRename } from '$lib/components/Layout/layoutStore';

import { isValidDropTarget, findItemById } from './dragHelpers';

// Auto-expand state
let hoverTimer: ReturnType<typeof setTimeout> | null = null;
let hoveredItemId: string | null = null;

/**
 * Handle drag start event
 */
export const createDragStartHandler = (ctx: 'fs' | 'git' | 'search') => {
	return (item: graph.FileNode | graph.FolderNode | graph.DBInstanceNode, event: DragEvent) => {
		if (ctx !== 'fs') return;

		let draggedIds: Set<string>;
		const currentSelection = get(selectedItemsStore);

		// If dragging a selected item, drag all selected items
		// Otherwise, drag only this item
		if (currentSelection.has(item.id)) {
			draggedIds = new Set(currentSelection);
		} else {
			draggedIds = new Set([item.id]);
			setItemSelection([item.id]);
		}

		// Start drag state
		startDrag(draggedIds);

		// Set drag data
		if (event.dataTransfer) {
			event.dataTransfer.effectAllowed = 'move';
			event.dataTransfer.setData('text/plain', Array.from(draggedIds).join(','));
		}
	};
};

/**
 * Clear auto-expand timer
 */
const clearAutoExpandTimer = () => {
	if (hoverTimer) {
		clearTimeout(hoverTimer);
		hoverTimer = null;
	}
	hoveredItemId = null;
};

/**
 * Handle drag over event with auto-expand
 */
export const createDragOverHandler = (ctx: 'fs' | 'git' | 'search') => {
	return (item: graph.FolderNode | graph.DBInstanceNode, event: DragEvent) => {
		const dragState = get(dragStateStore);
		if (ctx !== 'fs' || !dragState.isDragging) return;

		// Check if this is a valid drop target (not in the dragged selection)
		if (!isValidDropTarget(item.id, dragState.draggedItemIds)) return;

		event.preventDefault();
		event.stopPropagation();

		if (event.dataTransfer) {
			event.dataTransfer.dropEffect = 'move';
		}

		setHoveredTarget(item.id);

		// Auto-expand logic: if hovering over same item for 1 second, expand it
		if (hoveredItemId !== item.id) {
			// Clear previous timer if hovering over a different item
			clearAutoExpandTimer();
			hoveredItemId = item.id;

			// Set timer to expand after 1 second
			hoverTimer = setTimeout(() => {
				expandItem(item.id);
				hoverTimer = null;
			}, 750);
		}
	};
};

/**
 * Handle drop event
 */
export const createDropHandler = (ctx: 'fs' | 'git' | 'search') => {
	return async (targetItem: graph.FolderNode | graph.DBInstanceNode, event: DragEvent) => {
		const dragState = get(dragStateStore);
		if (ctx !== 'fs' || !dragState.isDragging) return;

		event.preventDefault();
		event.stopPropagation();

		// Clear auto-expand timer
		clearAutoExpandTimer();

		const draggedIds = dragState.draggedItemIds;

		// Validate drop target (not in dragged selection)
		if (!isValidDropTarget(targetItem.id, draggedIds)) {
			endDrag();
			return;
		}

		try {
			// Get root data for finding items
			const workspace = get(workspaceGraphStore);
			const rootFiles = workspace?.folders[0]?.files || [];
			const rootFolders = workspace?.folders[0]?.folders || [];
			const rootDatabases = workspace?.folders[0]?.db_instances || [];

			for (const id of draggedIds) {
				const item = findItemById(id, rootFiles, rootFolders, rootDatabases);
				if (!item) continue;

				// Calculate new URI (move item into target folder)
				const itemName = item.name;
				const targetUri = targetItem.uri;
				const newUri = `${targetUri}/${itemName}`;

				await must(
					tryCatch(Rename, {
						old_uri: item.uri,
						new_uri: newUri
					})
				);

				// Update open file tabs to point at the new path (files only)
				if (item.type === 'file') {
					updateFileTabsAfterRename(item.uri, newUri, itemName);
				}
			}

			expandItem(targetItem.id);

			notify({
				type: AlertType.Success,
				message: `Moved ${draggedIds.size} item${draggedIds.size > 1 ? 's' : ''} successfully`
			});
		} catch (error) {
			notifyError(`Failed to move items: ${error}`);
		} finally {
			endDrag();
		}
	};
};

/**
 * Handle drag end event
 */
export const createDragEndHandler = () => {
	return () => {
		clearAutoExpandTimer();
		endDrag();
	};
};

/**
 * Handle drag leave event
 */
export const createDragLeaveHandler = () => {
	return () => {
		setHoveredTarget(null);
	};
};

/**
 * Create all drag and drop handlers for a context
 */
export const createDragAndDropHandlers = (ctx: 'fs' | 'git' | 'search') => {
	return {
		handleDragStart: createDragStartHandler(ctx),
		handleDragOver: createDragOverHandler(ctx),
		handleDrop: createDropHandler(ctx),
		handleDragEnd: createDragEndHandler(),
		handleDragLeave: createDragLeaveHandler()
	};
};
