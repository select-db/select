import { writable, get } from 'svelte/store';
import { tryCatch } from '$lib/utils/tryCatch';
import { notify } from '$lib/system/Notifications/notificationsStore';
import { AlertType } from '$lib/system/Alert/types';
import {
	OpenFolder,
	PickFolder,
	ReopenLastFolder,
	GetLastFolder
} from '$lib/bindings/selectDb/internal/workspace/workspace';
import {
	OpenFolderResult,
	OpenFolderState,
	type LastFolder
} from '$lib/bindings/selectDb/internal/workspace/models';
import {
	clearWorkspaceGraphCache,
	initializeWorkspaceGraph,
	workspaceGraphStore
} from '$lib/utils/graph/workspaceGraphStore';

/** What the app shows between "signed in" and "here are your files". `null` while loading. */
export const folderStore = writable<OpenFolderResult | null>(null);

/** The folder offered on the no-folder screen. */
export const lastFolderStore = writable<LastFolder | null>(null);

function noFolder(): OpenFolderResult {
	return new OpenFolderResult({ state: OpenFolderState.OpenFolderNone });
}

export function clearFolderState() {
	folderStore.set(null);
	lastFolderStore.set(null);
}

/** Only the opened state has a workspace behind it; the rest are screens. */
async function apply(result: OpenFolderResult): Promise<void> {
	folderStore.set(result);
	clearWorkspaceGraphCache();

	if (result.state !== OpenFolderState.OpenFolderOpened) {
		await refreshLastFolder();
		return;
	}

	const [graph] = await tryCatch(initializeWorkspaceGraph);
	if (graph) workspaceGraphStore.set(graph);
}

export { apply as applyOpenResult };

export async function refreshLastFolder(): Promise<void> {
	const [last] = await tryCatch(GetLastFolder);
	lastFolderStore.set(last?.path ? last : null);
}

export async function openFolder(path: string): Promise<void> {
	const [result, err] = await tryCatch(OpenFolder, path);
	if (err) {
		notify({ type: AlertType.Error, message: err?.message ?? 'Could not open that folder' });
		return;
	}
	await apply(result);
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
		await apply(noFolder());
		return;
	}
	await apply(result);
}

/** The backend closed the folder on us, e.g. after a server delete. */
export async function onFolderClosed(): Promise<void> {
	if (get(folderStore)?.state === OpenFolderState.OpenFolderOpened) {
		notify({
			type: AlertType.Default,
			message: 'This workspace is no longer available to you. Your files were left untouched.',
			duration: 8000
		});
	}
	await apply(noFolder());
}
