import type * as graph from '$lib/wails/graph';

import { must, tryCatch } from '$lib/utils/tryCatch';
import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';

import type { ContextMenuOption } from '$lib/system/ContextMenu/types';

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
			action: async (onClose, { uri }: graph.FolderNode) => {
				await must(
					tryCatch(fs.Delete, {
						uri,
						recursive: true
					})
				);
				onClose();
			}
		}
	] satisfies ContextMenuOption[];
};

// Backward compatibility export
export const folderOptions = getFolderOptions('fs');
