import type { graph } from '$lib/wailsjs/go/models';
import { databaseActions } from './databaseActions';
import { getFileActions } from './fileActions';
import { getFolderActions } from './folderActions';

export const getActions = ({
	item,
	ctx = 'fs'
}: {
	item: graph.FileNode | graph.FolderNode | graph.DBInstanceNode | graph.DBInstanceItemNode;
	ctx?: 'fs' | 'git' | 'search';
}) => {
	if (item.type === 'file') return getFileActions(item as graph.FileNode, ctx);
	if (item.type === 'folder') return getFolderActions(item as graph.FolderNode, ctx);
	if (item.type === 'db_instance') return databaseActions;
	return [];
};
