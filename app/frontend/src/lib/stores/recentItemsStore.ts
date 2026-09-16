import { writable } from 'svelte/store';
import type { ResourceType } from '$lib/components/ResourceMenu/types';
import { tryCatch } from '$lib/utils/tryCatch';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';

const STORAGE_PREFIX = 'recentItems:';

// What every workspace wrote into before the list was scoped. Cleared on the
// first load so one workspace's files stop being offered while another is open.
const LEGACY_STORAGE_KEY = 'recentItems';

const MAX_ITEMS = 20;

export type RecentItem = {
	id: string;
	uri: string;
	type: ResourceType;
	name: string;
	folderId?: string;
	accessedAt: number;
};

/**
 * Whose list this is: the person and the workspace, the way the layout is
 * keyed. A machine two people sign in on is one browser profile, so the
 * workspace alone would hand one of them the other's files.
 */
let scope = '';

const storageKey = (id: string) => `${STORAGE_PREFIX}${id}`;

function loadFromStorage(id: string): RecentItem[] {
	if (typeof window === 'undefined' || !id) return [];
	const stored = localStorage.getItem(storageKey(id));
	if (!stored) return [];
	const [parsed] = tryCatch(JSON.parse, stored);
	return parsed ?? [];
}

function saveToStorage(id: string, items: RecentItem[]): void {
	if (typeof window === 'undefined' || !id) return;
	tryCatch(() => localStorage.setItem(storageKey(id), JSON.stringify(items)));
}

function dropLegacyStorage(): void {
	if (typeof window === 'undefined') return;
	tryCatch(() => localStorage.removeItem(LEGACY_STORAGE_KEY));
}

const createRecentItemsStore = () => {
	const { subscribe, set, update } = writable<RecentItem[]>([]);

	dropLegacyStorage();

	// The list belongs to one person in one workspace, so opening another
	// replaces it rather than adding to it: a file of the workspace that is no
	// longer open must not be offered by the search menu, let alone opened from
	// it.
	workspaceGraphStore.subscribe((workspace) => {
		const userId = workspace?.user?.id ?? '';
		const next = workspace?.id && userId ? `${userId}:${workspace.id}` : '';
		if (next === scope) return;
		scope = next;
		set(loadFromStorage(scope));
	});

	return {
		subscribe,

		addItem: (item: Omit<RecentItem, 'accessedAt'>) => {
			if (!scope) return;

			update((items) => {
				const filtered = items.filter((i) => i.id !== item.id);
				const newItem: RecentItem = { ...item, accessedAt: Date.now() };
				const updated = [newItem, ...filtered].slice(0, MAX_ITEMS);
				saveToStorage(scope, updated);
				return updated;
			});
		},

		removeItem: (id: string) => {
			update((items) => {
				const updated = items.filter((i) => i.id !== id);
				saveToStorage(scope, updated);
				return updated;
			});
		},

		clear: () => {
			set([]);
			saveToStorage(scope, []);
		}
	};
};

export const recentItemsStore = createRecentItemsStore();
