import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import type * as graph from '$lib/wails/graph';
import { navigateToFile } from '$lib/components/views/shared/navigateToFile';
import { navigateToDatabase } from '$lib/components/views/shared/navigateToDatabase';
import { expandItem, renamingItemIdStore } from '$lib/components/views/shared/sharedStore';

import { ResolveFolder } from '$lib/wails/graph';
import { notifyError } from '$lib/system/Notifications/notificationsStore';

import { createDatabase, writeFile, writeFolder } from './helpers';

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

// What a database is called before anyone renames it. The backend numbers it
// if a sibling already has the name, because the name is the directory.
const NEW_DATABASE_NAME = 'database';

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
	// Writing truncates, so the name has to miss every file already in there. A
	// db instance is resolved at build time and carries its own; a folder's are
	// read on demand, and a folder that will not resolve is not a folder to
	// write into.
	const files =
		folder.type === 'db_instance' ? folder.files : (await ResolveFolder(folder.uri))?.files;
	if (!files) {
		notifyError(`Could not read ${folder.name}`);
		return;
	}

	const name = findUniqueFileName(files);
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
		action: async (onClose, { uri, id }: graph.FolderNode) => {
			const created = await createDatabase(uri, NEW_DATABASE_NAME);
			navigateToDatabase({
				id: created.id,
				uri: created.uri,
				type: 'db_instance',
				name: created.name,
				folder_id: id,
				children: []
			} as unknown as graph.DBInstanceNode);
			expandItem(id);
			onClose();
		}
	}
] satisfies ContextMenuOption[];
