import { writable, get } from 'svelte/store';

import { ListHistory } from '$lib/wailsjs/go/history/History';
import type { history } from '$lib/wailsjs/go/models';

import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';

const PAGE_SIZE = 30;

export const historyItems = writable<history.HistoryEntry[]>([]);
export const historyLoading = writable(false);
export const historyDone = writable(false);

// Bumped whenever a statement is recorded so an open panel reloads its list.
export const historyRefreshNonce = writable(0);
export function bumpHistoryRefresh(): void {
	historyRefreshNonce.update((n) => n + 1);
}

let offset = 0;
let inFlight = false;

/** Clears the list and loads the first page for the current workspace. */
export async function resetHistory(): Promise<void> {
	offset = 0;
	historyItems.set([]);
	historyDone.set(false);
	await loadMoreHistory();
}

/**
 * Reloads the first page in place, swapping the list only once the data
 * arrives. Unlike resetHistory it never empties the list, so an open panel
 * doesn't flash to the loader/empty state when a new statement is recorded.
 */
export async function refreshHistory(): Promise<void> {
	const workspaceId = get(workspaceGraphStore)?.id;
	if (!workspaceId) {
		historyItems.set([]);
		historyDone.set(true);
		return;
	}
	if (inFlight) return;

	inFlight = true;
	historyLoading.set(true);
	try {
		const batch = await ListHistory({ workspaceId, limit: PAGE_SIZE, offset: 0 });
		offset = batch.length;
		historyDone.set(batch.length < PAGE_SIZE);
		historyItems.set(batch);
	} catch {
		// Keep the existing list on a transient read error rather than blanking it.
	} finally {
		inFlight = false;
		historyLoading.set(false);
	}
}

/** Loads the next page, appending to the list. No-op once exhausted. */
export async function loadMoreHistory(): Promise<void> {
	if (inFlight || get(historyDone)) return;

	const workspaceId = get(workspaceGraphStore)?.id;
	if (!workspaceId) {
		historyDone.set(true);
		return;
	}

	inFlight = true;
	historyLoading.set(true);
	try {
		const batch = await ListHistory({ workspaceId, limit: PAGE_SIZE, offset });
		historyItems.update((items) => [...items, ...batch]);
		offset += batch.length;
		if (batch.length < PAGE_SIZE) historyDone.set(true);
	} catch {
		// Local convenience log: swallow read errors and stop paging.
		historyDone.set(true);
	} finally {
		inFlight = false;
		historyLoading.set(false);
	}
}
