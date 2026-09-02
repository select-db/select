import { must, tryCatch } from '$lib/utils/tryCatch';
import { clearResolvedFolders } from '$lib/utils/graph/resolveFolder';
import { invalidateWorkspaceFiles } from '$lib/utils/graph/workspaceFiles';
import { GetWorkspaceGraph } from '$lib/wails/graph';
import type * as graph from '$lib/wails/graph';
import { writable, get } from 'svelte/store';

export const workspaceGraphStore = writable<graph.WorkspaceNode | undefined>();

let loading: Promise<graph.WorkspaceNode | null> | null = null;

/** Clears cached graph and any in-flight load. Call when switching server so the next init fetches for the current server. */
export function clearWorkspaceGraphCache() {
	workspaceGraphStore.set(undefined);
	loading = null;
	clearResolvedFolders();
	invalidateWorkspaceFiles();
}

export const initializeWorkspaceGraph = async () => {
	const currentGraph = get(workspaceGraphStore);
	if (currentGraph) return currentGraph;

	if (loading) return loading;

	loading = (async () => {
		const g = await must(tryCatch(GetWorkspaceGraph));
		workspaceGraphStore.set(g ?? undefined);
		return g;
	})();

	return loading;
};
