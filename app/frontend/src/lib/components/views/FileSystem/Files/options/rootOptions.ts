import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import type * as graph from '$lib/wails/graph';
import { navigateToFile } from '$lib/components/views/shared/navigateToFile';
import { navigateToDatasource } from '$lib/components/views/shared/navigateToDatasource';
import { expandItem, renamingItemIdStore } from '$lib/components/views/shared/sharedStore';

import { ResolveFolder } from '$lib/wails/graph';
import { notifyError } from '$lib/system/Notifications/notificationsStore';

import { writeDatasource, writeFile, writeFolder } from './helpers';

type FolderLike = graph.FolderNode | graph.DatasourceNode;

/**
 * Every name the folder is already using, whatever kind of thing is using it.
 *
 * One namespace, because a directory has one: a datasource is a directory,
 * so a datasource named for a folder that is already there does not fail, it
 * writes a datasource.config.json into that folder and takes it over.
 */
const namesInFolder = (folder: FolderLike): Set<string> =>
	new Set(
		[
			...('folders' in folder ? folder.folders : []),
			...('files' in folder ? (folder.files ?? []) : []),
			...('datasources' in folder ? folder.datasources : [])
		].map((entry) => entry.name)
	);

/** `pattern(1)`, `pattern(2)` ... for the first the folder is not using. */
const findUniqueName = (taken: Set<string>, pattern: (n: number) => string): string => {
	let counter = 1;
	while (taken.has(pattern(counter))) {
		counter++;
	}
	return pattern(counter);
};

export const createFolderInFolder = async (folder: FolderLike) => {
	const name = findUniqueName(namesInFolder(folder), (n) => `folder #${n}`);
	const uri = `${folder.uri}/${name}`;
	await writeFolder(uri);
	expandItem(folder.id);
	renamingItemIdStore.set(uri);
};

export const createFileInFolder = async (folder: FolderLike) => {
	// Writing truncates, so the name has to miss every file already in there. A
	// datasource is resolved at build time and carries its own; a folder's are
	// read on demand, and a folder that will not resolve is not a folder to
	// write into.
	const resolved = folder.type === 'datasource' ? folder : await ResolveFolder(folder.uri);
	if (!resolved?.files) {
		notifyError(`Could not read ${folder.name}`);
		return;
	}

	const name = findUniqueName(namesInFolder(resolved), (n) => `#${n}.sql`);
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
		action: async (onClose, folder: graph.FolderNode) => {
			const { uri, id } = folder;
			const name = findUniqueName(namesInFolder(folder), (n) => `db #${n}`);
			const { id: datasourceId, uri: datasourceUri } = await writeDatasource(uri, name);
			navigateToDatasource({
				id: datasourceId,
				uri: datasourceUri,
				type: 'datasource',
				name,
				folder_id: id,
				children: []
			} as unknown as graph.DatasourceNode);
			expandItem(id);
			onClose();
		}
	}
] satisfies ContextMenuOption[];
