import { writable } from 'svelte/store';

import { Replace } from '$lib/bindings/selectDb/internal/search/search';
import { SearchWithNodes, type SearchResultWithNodes } from '$lib/wails/graph';
import { EventsEmit } from '$lib/wails/events';
import { expandItem } from '../shared/sharedStore';

export const searchResultsStore = writable<SearchResultWithNodes | null>(null);

export interface SearchParams {
	workspaceId: string;
	pattern: string;
	useRegex: boolean;
	caseSensitive: boolean;
	wholeWord: boolean;
	includePattern: string;
	excludePattern: string;
}

export interface ReplaceParams extends SearchParams {
	replacement: string;
	filePath: string;
	dryRun: boolean;
}

export async function performSearch(params: SearchParams) {
	const result = await SearchWithNodes(params);
	searchResultsStore.set(result);
	const root = result?.resultFolder;
	if (!root) return result;
	for (const folder of root.folders) expandItem(folder.id);
	return result;
}

export async function performReplace(params: ReplaceParams) {
	const result = await Replace(params);

	// Notify editors that files have been modified on disk
	if (result?.modifiedFiles?.length) {
		const fileURIs = result.modifiedFiles.map(
			(path) => `selectdb://workspaces/${params.workspaceId}/${path}`
		);
		EventsEmit('files:modified', fileURIs);
	}

	return result;
}

export function clearSearchResults() {
	searchResultsStore.set(null);
}
