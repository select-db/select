import type * as generated from '$lib/bindings/selectDb/internal/db/generated/models';
import type * as graph from '$lib/wails/graph';
import { EventsOn } from '$lib/wails/events';
import { resolveFolderContents } from '$lib/utils/graph/resolveFolder';
import { writable } from 'svelte/store';
import {
	createDatabase,
	createFile,
	createFolder,
	deleteDatabase,
	deleteFile,
	deleteFolder,
	updateDatabase,
	updateFile
} from './assistant/all';

export type FileSystemItem = graph.FileNode | graph.FolderNode;

export const expandedItemIdsStore = writable<Map<string, boolean>>(new Map([]));

export const toggleIsItemExpanded = (id: string) => {
	let expanded = false;

	expandedItemIdsStore.update((store) => {
		const newStore = new Map(store);
		expanded = !newStore.get(id);
		newStore.set(id, expanded);
		return newStore;
	});

	if (expanded) resolveFolderContents(id);
};

export const expandItem = (id: string) => {
	expandedItemIdsStore.update((store) => {
		const newStore = new Map(store);
		newStore.set(id, true);
		return newStore;
	});

	resolveFolderContents(id);
};

export const renamingItemIdStore = writable<string | null>(null);

export const lastVisibleItemIdStore = writable<string | null>(null);

EventsOn('mutation', (commit: generated.MutationCommit) => {
	// File
	createFile(commit);
	updateFile(commit);
	deleteFile(commit);
	// Folder
	createFolder(commit);
	deleteFolder(commit);
	// Database
	createDatabase(commit);
	updateDatabase(commit);
	deleteDatabase(commit);
});

/**
 * Selection for drag and drop
 */
export const selectedItemsStore = writable<Set<string>>(new Set());

export const toggleItemSelection = (id: string) => {
	selectedItemsStore.update((set) => {
		const newSet = new Set(set);
		if (newSet.has(id)) {
			newSet.delete(id);
		} else {
			newSet.add(id);
		}
		return newSet;
	});
};

export const removeFromItemSelection = (id: string) => {
	selectedItemsStore.update((set) => {
		const newSet = new Set(set);
		if (newSet.has(id)) newSet.delete(id);
		return newSet;
	});
};

export const addToItemSelection = (id: string) => {
	selectedItemsStore.update((set) => {
		const newSet = new Set(set);
		newSet.add(id);
		return newSet;
	});
};

export const clearItemSelection = () => {
	selectedItemsStore.set(new Set());
};

export const setItemSelection = (ids: string[]) => {
	selectedItemsStore.set(new Set(ids));
};

/**
 * Keyboard cursor for file system panel
 */
export const focusedFsItemStore = writable<string | null>(null);
export const setFocusedFsItem = (id: string | null) => focusedFsItemStore.set(id);

export const fsPanelFocusSignal = writable(0);
export const requestFsPanelFocus = () => fsPanelFocusSignal.update((n) => n + 1);

/**
 * Drag and drop state
 */
type DragState = {
	isDragging: boolean;
	draggedItemIds: Set<string>;
	hoveredTargetId: string | null;
};

const initialDragState: DragState = {
	isDragging: false,
	draggedItemIds: new Set(),
	hoveredTargetId: null
};

export const dragStateStore = writable<DragState>(initialDragState);

export const startDrag = (draggedIds: Set<string>) => {
	dragStateStore.set({
		isDragging: true,
		draggedItemIds: draggedIds,
		hoveredTargetId: null
	});
};

export const setHoveredTarget = (id: string | null) => {
	dragStateStore.update((state) => ({
		...state,
		hoveredTargetId: id
	}));
};

export const endDrag = () => {
	dragStateStore.set(initialDragState);
};
