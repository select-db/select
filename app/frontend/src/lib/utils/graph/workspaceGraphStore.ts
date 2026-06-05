import { must, tryCatch } from '$lib/utils/tryCatch';
import { GetWorkspaceGraph } from '$lib/wailsjs/go/graph/Graph';
import type { graph } from '$lib/wailsjs/go/models';
import { writable, get } from 'svelte/store';

export const workspaceGraphStore = writable<graph.WorkspaceNode | undefined>();

let loading: Promise<graph.WorkspaceNode> | null = null;

/** Clears cached graph and any in-flight load. Call when switching server so the next init fetches for the current server. */
export function clearWorkspaceGraphCache() {
	workspaceGraphStore.set(undefined);
	loading = null;
}

export const initializeWorkspaceGraph = async () => {
	const currentGraph = get(workspaceGraphStore);
	if (currentGraph) return currentGraph;

	if (loading) return loading;

	loading = (async () => {
		const g = await must(tryCatch(GetWorkspaceGraph));
		workspaceGraphStore.set(g);
		return g;
	})();

	return loading;
};
