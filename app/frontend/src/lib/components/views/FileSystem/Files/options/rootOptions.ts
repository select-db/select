import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import type * as graph from '$lib/wails/graph';
import { navigateToFile } from '$lib/components/views/shared/navigateToFile';
import { navigateToDatabase } from '$lib/components/views/shared/navigateToDatabase';
import { expandItem, renamingItemIdStore } from '$lib/components/views/shared/sharedStore';

import { writeDatabase, writeFile, writeFolder } from './helpers';

const findUniqueFolderName = (folders: graph.FolderNode[]): string => {
	const existingNames = new Set(folders.map((f) => f.name));
	let counter = 1;
	while (existingNames.has(`folder #${counter}`)) {
		counter++;
	}
	return `folder #${counter}`;
};

const findUniqueFileName = (files: graph.FileNode[]): string => {
	const existingNames = new Set(files.map((f) => f.name));
	let counter = 1;
	while (existingNames.has(`#${counter}.sql`)) {
		counter++;
	}
	return `#${counter}.sql`;
};

const findUniqueDatabaseName = (databases: graph.DBInstanceNode[]): string => {
	const existingNames = new Set(databases.map((db) => db.name));
	let counter = 1;
	while (existingNames.has(`db #${counter}`)) {
		counter++;
	}
	return `db #${counter}`;
};

type FolderLike = graph.FolderNode | graph.DBInstanceNode;

export const createFolderInFolder = async (folder: FolderLike) => {
	const folders = 'folders' in folder ? folder.folders : [];
	const name = findUniqueFolderName(folders);
	const uri = `${folder.uri}/${name}`;
	await writeFolder(uri);
	expandItem(folder.id);
	renamingItemIdStore.set(uri);
};

export const createFileInFolder = async (folder: FolderLike) => {
	const name = findUniqueFileName(folder.files);
	const fileUri = `${folder.uri}/${name}`;
	await writeFile(fileUri);
	navigateToFile({
		id: fileUri,
		uri: fileUri,
		type: 'file',
		name,
		folder_id: folder.uri
	} as graph.FileNode);
	expandItem(folder.id);
	renamingItemIdStore.set(fileUri);
};

export const rootOptions = [
	{
		label: 'New folder...',
		action: async (onClose, folder: graph.FolderNode) => {
			await createFolderInFolder(folder);
			onClose();
		}
	},
	{
		label: 'New file...',
		action: async (onClose, folder: graph.FolderNode) => {
			await createFileInFolder(folder);
			onClose();
		}
	},
	{
		label: 'New Database...',
		action: async (onClose, { uri, id, db_instances }: graph.FolderNode) => {
			const name = findUniqueDatabaseName(db_instances);
			const { id: dbId, uri: dbUri } = await writeDatabase(uri, name);
			navigateToDatabase({
				id: dbId,
				uri: dbUri,
				type: 'db_instance',
				name,
				folder_id: id,
				children: []
			} as unknown as graph.DBInstanceNode);
			expandItem(id);
			onClose();
		}
	}
] satisfies ContextMenuOption[];
