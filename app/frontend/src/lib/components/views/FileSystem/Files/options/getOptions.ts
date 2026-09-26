import { get } from 'svelte/store';
import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import type * as graph from '$lib/wails/graph';
import { getDatasourceItemOptions } from './datasourceItemOptions';
import { datasourceOptions } from './datasourceOptions';
import { getFileOptions } from './fileOptions';
import { getFolderOptions } from './folderOptions';
import { selectedItemsStore } from '$lib/components/views/shared/sharedStore';
import { batchDeleteOptions } from './batchDeleteOptions';

export const getOptions = (
	item: graph.FileNode | graph.FolderNode | graph.DatasourceNode | graph.DatasourceItemNode,
	ctx?: 'fs' | 'git' | 'search'
): ContextMenuOption[] => {
	const selectedItems = get(selectedItemsStore);
	if (selectedItems.size > 1) return batchDeleteOptions;

	if (item.type === 'folder') return getFolderOptions(ctx || 'fs');
	if (item.type === 'file') return getFileOptions(item as graph.FileNode, ctx || 'fs');
	if (item.type === 'datasource') return datasourceOptions;
	return getDatasourceItemOptions(item as graph.DatasourceItemNode);
};
