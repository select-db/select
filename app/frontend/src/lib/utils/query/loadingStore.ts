import { writable } from 'svelte/store';

export const loadingStore = writable<Array<string>>([]);

export const toKey = (databaseId?: string, fileId?: string): string => {
	return `${databaseId ? `db:${databaseId}` : ''}${fileId ? `:file:${fileId}` : ''}`;
};

export const pushToLoadingStore = (databaseId?: string, fileId?: string) => {
	const key = toKey(databaseId, fileId);
	loadingStore.update((state) => {
		if (state.includes(key)) return state;
		return [...state, key];
	});
};

export const removeFromLoadingStore = (databaseId?: string, fileId?: string) => {
	const key = toKey(databaseId, fileId);
	loadingStore.update((state) => state.filter((i) => i !== key));
};
