import { writable } from 'svelte/store';

/**
 * What is running, for display only.
 */
export const loadingStore = writable<Array<string>>([]);

export const toKey = (dbInstanceId?: string, fileId?: string): string => {
	return `${dbInstanceId ? `db:${dbInstanceId}` : ''}${fileId ? `:file:${fileId}` : ''}`;
};

export const pushToLoadingStore = (dbInstanceId?: string, fileId?: string) => {
	const key = toKey(dbInstanceId, fileId);
	loadingStore.update((state) => {
		if (state.includes(key)) return state;
		return [...state, key];
	});
};

export const removeFromLoadingStore = (dbInstanceId?: string, fileId?: string) => {
	const key = toKey(dbInstanceId, fileId);
	loadingStore.update((state) => state.filter((i) => i !== key));
};
