import { get } from 'svelte/store';
import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import { selectedItemsStore, clearItemSelection } from '$lib/components/views/shared/sharedStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { findItemById } from '../helpers/dragHelpers';
import type * as graph from '$lib/wails/graph';
import { deleteEntries } from '$lib/components/views/shared/deleteEntries';

export const batchDeleteOptions: ContextMenuOption[] = [
	{
		label: 'Delete selected',
		action: async (onClose) => {
			const selected = get(selectedItemsStore);
			const graph = get(workspaceGraphStore);
			if (!graph) return onClose?.();

			const items: (graph.FileNode | graph.FolderNode | graph.DBInstanceNode)[] = [];
			for (const id of selected) {
				const item = findItemById(id, [], graph.folders, graph.db_instances);
				if (item) items.push(item);
			}

			await deleteEntries(items, () =>
				notifyError(`${selected.size} ${selected.size === 1 ? 'item' : 'items'} deleted`)
			);

			clearItemSelection();
			onClose?.();
		}
	}
];
