import { writable, get } from 'svelte/store';
import { tryCatch } from '$lib/utils/tryCatch';
import { notify } from '$lib/system/Notifications/notificationsStore';
import { AlertType } from '$lib/system/Alert/types';
import {
	OpenFolder,
	PickFolder,
	ReopenLastFolder,
	ListFolders
} from '$lib/bindings/selectDb/internal/workspace/workspace';
import {
	FolderState,
	WorkspaceStatus,
	type Folder
} from '$lib/bindings/selectDb/internal/workspace/models';
import {
	clearWorkspaceGraphCache,
	initializeWorkspaceGraph
} from '$lib/utils/graph/workspaceGraphStore';

/** What the app shows between "signed in" and "here are your files". `null` while loading. */
export const folderStore = writable<FolderState | null>(null);

/** The folders this machine has, offered on the no-folder screen. */
export const foldersStore = writable<Folder[]>([]);

/**
 * True while a folder is being opened, which the buttons that open one show as
 * loading. It covers the open and never the picker: a spinner tied to an OS
 * dialog is one the app cannot clear when the dialog answers nothing.
 */
export const openingStore = writable(false);

function noFolder(): FolderState {
	return new FolderState({ status: WorkspaceStatus.NoFolder });
}

export function clearFolderState(): void {
	folderStore.set(null);
	foldersStore.set([]);
}

/** Displays the folder: its tree when ready, the screen for its status otherwise. */
export async function displayFolder(state: FolderState): Promise<void> {
	folderStore.set(state);
	clearWorkspaceGraphCache();

	if (state.status === WorkspaceStatus.Ready) {
		// Sets the store itself; called for its error handling.
		await tryCatch(initializeWorkspaceGraph);
		return;
	}

	await refreshFolders();
}

export async function refreshFolders(): Promise<void> {
	const [folders] = await tryCatch(ListFolders);
	foldersStore.set(folders ?? []);
}

export async function openFolder(path: string): Promise<void> {
	openingStore.set(true);
	const [result, err] = await tryCatch(OpenFolder, path);
	openingStore.set(false);

	if (err) {
		notify({ type: AlertType.Error, message: err?.message ?? 'Could not open that folder' });
		return;
	}
	await displayFolder(result);
}

export async function pickAndOpenFolder(): Promise<void> {
	const [path, err] = await tryCatch(PickFolder);
	if (err) {
		notify({ type: AlertType.Error, message: err?.message ?? 'Could not open the folder picker' });
		return;
	}
	if (!path) return; // cancelled
	await openFolder(path);
}

/** Runs after login, so a returning user skips the picker. */
export async function reopenLastFolder(): Promise<void> {
	const [result, err] = await tryCatch(ReopenLastFolder);
	if (err) {
		// No notification: the user did not ask for this, and the no-folder
		// screen is a fine place to land.
		await displayFolder(noFolder());
		return;
	}
	await displayFolder(result);
}

/** The backend closed the folder on us, e.g. after a server delete. */
export async function onFolderClosed(): Promise<void> {
	if (get(folderStore)?.status === WorkspaceStatus.Ready) {
		notify({
			type: AlertType.Default,
			message: 'This workspace is no longer available to you. Your files were left untouched.',
			duration: 8000
		});
	}
	await displayFolder(noFolder());
}
