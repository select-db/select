import { writable } from 'svelte/store';

const STORAGE_KEY = 'fileSystem.hiddenChildren';

// parentId -> list of hidden child ids under that parent
type HiddenMap = Record<string, string[]>;

function load(): HiddenMap {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return {};
		const parsed = JSON.parse(raw);
		if (parsed && typeof parsed === 'object') return parsed as HiddenMap;
		return {};
	} catch {
		return {};
	}
}

function save(map: HiddenMap): void {
	localStorage.setItem(STORAGE_KEY, JSON.stringify(map));
}

export const hiddenChildrenStore = writable<HiddenMap>(load());

export function toggleChildVisibility(parentId: string, childId: string): void {
	hiddenChildrenStore.update((map) => {
		const current = new Set(map[parentId] ?? []);
		if (current.has(childId)) current.delete(childId);
		else current.add(childId);

		const next = { ...map };
		if (current.size === 0) delete next[parentId];
		else next[parentId] = [...current];

		save(next);
		return next;
	});
}

export function filterVisibleChildren<T extends { id: string }>(
	parentId: string,
	children: T[],
	hidden: HiddenMap
): T[] {
	const list = hidden[parentId];
	if (!list || list.length === 0) return children;
	const set = new Set(list);
	return children.filter((c) => !set.has(c.id));
}
