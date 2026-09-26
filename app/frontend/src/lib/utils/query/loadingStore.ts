import { writable } from 'svelte/store';

/**
 * What is running, for display only.
 */
export const loadingStore = writable<Array<string>>([]);

export const toKey = (datasourceId?: string, fileId?: string): string => {
	return `${datasourceId ? `db:${datasourceId}` : ''}${fileId ? `:file:${fileId}` : ''}`;
};

export const pushToLoadingStore = (datasourceId?: string, fileId?: string) => {
	const key = toKey(datasourceId, fileId);
	loadingStore.update((state) => {
		if (state.includes(key)) return state;
		return [...state, key];
	});
};

export const removeFromLoadingStore = (datasourceId?: string, fileId?: string) => {
	const key = toKey(datasourceId, fileId);
	loadingStore.update((state) => state.filter((i) => i !== key));
};
