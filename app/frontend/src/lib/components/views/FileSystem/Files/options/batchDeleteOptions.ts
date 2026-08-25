import { get } from 'svelte/store';
import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import { selectedItemsStore, clearItemSelection } from '$lib/components/views/shared/sharedStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { findItemById } from '../helpers/dragHelpers';
import type * as graph from '$lib/wails/graph';
import { must, tryCatch } from '$lib/utils/tryCatch';
import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';

export const batchDeleteOptions: ContextMenuOption[] = [
	{
		label: 'Delete selected',
		action: async (onClose) => {
			const selected = get(selectedItemsStore);
			const graph = get(workspaceGraphStore);
			if (!graph) return onClose?.();

			for (const id of selected) {
				const item = findItemById(id, [], graph.folders, graph.db_instances);
				if (item) await deleteItem(item);
			}

			notifyError(`${selected.size} ${selected.size === 1 ? 'item' : 'items'} deleted`);
			clearItemSelection();
			onClose?.();
		}
	}
];

export const deleteItem = async (
	item: graph.FileNode | graph.FolderNode | graph.DBInstanceNode
) => {
	if (item.type === 'file') {
		await must(tryCatch(fs.Delete, { uri: item.uri, recursive: false }));
		await tryCatch(fs.Delete, { uri: item.uri + '.metadata.json', recursive: false });
	} else {
		await must(tryCatch(fs.Delete, { uri: item.uri, recursive: true }));
	}
};
