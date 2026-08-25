import { writable, derived } from 'svelte/store';
import type * as graph from '$lib/wails/graph';

const ITEM_HEIGHT = 30;
const OVERSCAN = 50;

type Context = 'fs' | 'search' | 'git';

interface VisibilityIndex {
	flatIds: string[];
	descendantRange: Map<string, [number, number]>;
}

interface ContextState {
	index: VisibilityIndex;
	window: [number, number];
}

const emptyIndex: VisibilityIndex = { flatIds: [], descendantRange: new Map() };

const contextStatesStore = writable<Map<Context, ContextState>>(
	new Map([
		['fs', { index: emptyIndex, window: [0, 50] }],
		['search', { index: emptyIndex, window: [0, 50] }],
		['git', { index: emptyIndex, window: [0, 50] }]
	])
);

export const fsVisibilityIndexStore = derived(
	contextStatesStore,
	($states) => $states.get('fs')?.index ?? emptyIndex
);

// Visible IDs across all contexts: window items + containers whose descendants overlap the window.
export const visibleIdsStore = derived(contextStatesStore, ($states) => {
	const visible = new Set<string>();

	for (const [, state] of $states) {
		const { flatIds, descendantRange } = state.index;
		const [windowStart, windowEnd] = state.window;

		const start = Math.max(0, windowStart);
		const end = Math.min(flatIds.length, windowEnd);
		for (let i = start; i < end; i++) {
			visible.add(flatIds[i]);
		}

		for (const [id, [minChild, maxChild]] of descendantRange) {
			if (minChild < windowEnd && maxChild >= windowStart) {
				visible.add(id);
			}
		}
	}

	return visible;
});

export function buildVisibilityIndex(
	ctx: Context,
	folders: graph.FolderNode[],
	files: graph.FileNode[],
	databases: graph.DBInstanceNode[],
	databaseItems: graph.DBInstanceItemNode[],
	expandedIds: Map<string, boolean>,
	hiddenChildren: Record<string, string[]> = {}
): void {
	const flatIds: string[] = [];
	const descendantRange = new Map<string, [number, number]>();

	const recordRange = (id: string, firstChildIndex: number) => {
		if (flatIds.length > firstChildIndex) {
			descendantRange.set(id, [firstChildIndex, flatIds.length - 1]);
		}
	};

	const visibleChildren = <T extends { id: string }>(parentId: string, children: T[]): T[] => {
		const list = hiddenChildren[parentId];
		if (!list || list.length === 0) return children;
		const set = new Set(list);
		return children.filter((c) => !set.has(c.id));
	};

	const walkDatabaseItems = (items: graph.DBInstanceItemNode[]): void => {
		for (const item of items) {
			const myIndex = flatIds.length;
			flatIds.push(item.id);

			if (expandedIds.get(item.id) && item.children?.length) {
				walkDatabaseItems(visibleChildren(item.id, item.children));
			}

			recordRange(item.id, myIndex + 1);
		}
	};

	const walk = (
		folders: graph.FolderNode[],
		files: graph.FileNode[],
		databases: graph.DBInstanceNode[],
		databaseItems: graph.DBInstanceItemNode[]
	): void => {
		for (const db of databases) {
			const myIndex = flatIds.length;
			flatIds.push(db.id);

			if (expandedIds.get(db.id)) {
				walk(db.folders, db.files, [], visibleChildren(db.id, db.children));
			}

			recordRange(db.id, myIndex + 1);
		}

		for (const folder of folders) {
			const myIndex = flatIds.length;
			flatIds.push(folder.id);

			if (expandedIds.get(folder.id)) {
				walk(folder.folders, folder.files, folder.db_instances, []);
			}

			recordRange(folder.id, myIndex + 1);
		}

		walkDatabaseItems(databaseItems);

		for (const file of files) {
			flatIds.push(file.id);
		}
	};

	walk(folders, files, databases, databaseItems);

	contextStatesStore.update((states) => {
		const newStates = new Map(states);
		const current = newStates.get(ctx)!;
		newStates.set(ctx, { ...current, index: { flatIds, descendantRange } });
		return newStates;
	});
}

export function updateScrollWindow(ctx: Context, scrollTop: number, viewportHeight: number): void {
	const start = Math.max(0, ((scrollTop / ITEM_HEIGHT) | 0) - OVERSCAN);
	const end = start + ((viewportHeight / ITEM_HEIGHT) | 0) + OVERSCAN * 2 + 1;

	contextStatesStore.update((states) => {
		const current = states.get(ctx)!;
		if (current.window[0] === start && current.window[1] === end) return states;
		const newStates = new Map(states);
		newStates.set(ctx, { ...current, window: [start, end] });
		return newStates;
	});
}
