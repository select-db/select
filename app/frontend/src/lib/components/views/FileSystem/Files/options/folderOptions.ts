import type * as graph from '$lib/wails/graph';

import type { ContextMenuOption } from '$lib/system/ContextMenu/types';

import { deleteEntries } from '$lib/components/views/shared/deleteEntries';
import { rootOptions } from './rootOptions';
import { renameOption } from './helpers';

export const getFolderOptions = (ctx: 'fs' | 'git' | 'search' = 'fs'): ContextMenuOption[] => {
	// For git context, return empty options (placeholder folders shouldn't have context menu)
	if (ctx !== 'fs') return [];

	return [
		...rootOptions,
		renameOption,
		{
			label: '',
			divider: true
		},
		{
			label: 'Delete',
			action: async (onClose, folder: graph.FolderNode) => {
				await deleteEntries([folder]);
				onClose();
			}
		}
	] satisfies ContextMenuOption[];
};

// Backward compatibility export
export const folderOptions = getFolderOptions('fs');
