import { get } from 'svelte/store';
import type { graph } from '$lib/wailsjs/go/models';
import { getDatabaseItemOptions } from './databaseItemOptions';
import { databaseOptions } from './databaseOptions';
import { getFileOptions } from './fileOptions';
import { getFolderOptions } from './folderOptions';
import { selectedItemsStore } from '$lib/components/views/shared/sharedStore';
import { batchDeleteOptions } from './batchDeleteOptions';

export const getOptions = (
	item: graph.FileNode | graph.FolderNode | graph.DBInstanceNode | graph.DBInstanceItemNode,
	ctx?: 'fs' | 'git' | 'search'
) => {
	const selectedItems = get(selectedItemsStore);
	if (selectedItems.size > 1) return batchDeleteOptions;

	if (item.type === 'folder') return getFolderOptions(ctx || 'fs');
	if (item.type === 'file') return getFileOptions(item as graph.FileNode, ctx || 'fs');
	if (item.type === 'db_instance') return databaseOptions;
	return getDatabaseItemOptions(item as graph.DBInstanceItemNode);
};
