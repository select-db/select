import type * as graph from '$lib/wails/graph';
import { datasourceActions } from './datasourceActions';
import { getFileActions } from './fileActions';
import { getFolderActions } from './folderActions';

export const getActions = ({
	item,
	ctx = 'fs'
}: {
	item: graph.FileNode | graph.FolderNode | graph.DatasourceNode | graph.DatasourceItemNode;
	ctx?: 'fs' | 'git' | 'search';
}) => {
	if (item.type === 'file') return getFileActions(item as graph.FileNode, ctx);
	if (item.type === 'folder') return getFolderActions(item as graph.FolderNode, ctx);
	if (item.type === 'datasource') return datasourceActions;
	return [];
};
