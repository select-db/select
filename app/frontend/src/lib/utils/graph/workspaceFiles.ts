import { ListWorkspaceFiles, type FileNode } from '$lib/wails/graph';
import { tryCatch } from '$lib/utils/tryCatch';
import { writable, get } from 'svelte/store';

/**
 * Every file in the workspace, for the pickers that search it by name.
 *
 * The workspace graph holds the files of the folders that have been opened —
 * what the tree shows — so it is the wrong source for a picker, which is asked
 * about files the user has never browsed to. The backend walks for those on
 * request; this holds the answer until something changes it.
 *
 * It is loaded when a picker first asks, not with the workspace: the walk is
 * only worth paying for if someone opens a picker.
 */
export const workspaceFilesStore = writable<FileNode[]>([]);

let loaded = false;
let loading: Promise<void> | null = null;

export function loadWorkspaceFiles(): Promise<void> {
	if (loaded) return Promise.resolve();
	if (loading) return loading;

	loading = (async () => {
		const [files, err] = await tryCatch(ListWorkspaceFiles);
		loading = null;
		if (err) return;

		workspaceFilesStore.set(files ?? []);
		loaded = true;
	})();

	return loading;
}

/**
 * Drops the list so the next picker reloads it. Called when files are added or
 * removed, and when the workspace itself is replaced.
 */
export function invalidateWorkspaceFiles() {
	if (!loaded && get(workspaceFilesStore).length === 0) return;
	loaded = false;
	workspaceFilesStore.set([]);
}
