import { writable } from 'svelte/store';
import type { ResourceType } from '$lib/components/ResourceMenu/types';
import { tryCatch } from '$lib/utils/tryCatch';

const STORAGE_KEY = 'recentItems';
const MAX_ITEMS = 20;

export type RecentItem = {
	id: string;
	uri: string;
	type: ResourceType;
	name: string;
	folderId?: string;
	accessedAt: number;
};

function loadFromStorage(): RecentItem[] {
	if (typeof window === 'undefined') return [];
	const stored = localStorage.getItem(STORAGE_KEY);
	if (!stored) return [];
	const [parsed] = tryCatch(JSON.parse, stored);
	return parsed ?? [];
}

function saveToStorage(items: RecentItem[]): void {
	if (typeof window === 'undefined') return;
	tryCatch(() => localStorage.setItem(STORAGE_KEY, JSON.stringify(items)));
}

const createRecentItemsStore = () => {
	const { subscribe, set, update } = writable<RecentItem[]>(loadFromStorage());

	return {
		subscribe,

		addItem: (item: Omit<RecentItem, 'accessedAt'>) => {
			update((items) => {
				const filtered = items.filter((i) => i.id !== item.id);
				const newItem: RecentItem = { ...item, accessedAt: Date.now() };
				const updated = [newItem, ...filtered].slice(0, MAX_ITEMS);
				saveToStorage(updated);
				return updated;
			});
		},

		removeItem: (id: string) => {
			update((items) => {
				const updated = items.filter((i) => i.id !== id);
				saveToStorage(updated);
				return updated;
			});
		},

		clear: () => {
			set([]);
			saveToStorage([]);
		}
	};
};

export const recentItemsStore = createRecentItemsStore();
