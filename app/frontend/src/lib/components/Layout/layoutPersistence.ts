import type * as graph from '$lib/wails/graph';
import type { Layout } from './layoutStore';
import { layoutStore, createInitialLayout } from './layoutStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { tryCatch } from '$lib/utils/tryCatch';

const STORAGE_KEY_PREFIX = 'selectdb:layout:';
const SAVE_DEBOUNCE_MS = 500;

function getStorageKey(userId: string, workspaceId: string): string {
	return `${STORAGE_KEY_PREFIX}${userId}:${workspaceId}`;
}

function isSerializable(value: unknown): boolean {
	if (value === null) return true;
	if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean')
		return true;
	if (
		typeof value === 'undefined' ||
		typeof value === 'function' ||
		typeof value === 'symbol' ||
		typeof value === 'bigint'
	)
		return false;
	if (value instanceof Set || value instanceof Map) return false;
	return true;
}

/** Drop non-JSON-serializable values so we can stringify. */
function toSerializable(value: unknown): unknown {
	if (!isSerializable(value)) return undefined;
	if (Array.isArray(value)) return value.map(toSerializable).filter((x) => x !== undefined);
	if (value !== null && typeof value === 'object') {
		const out: Record<string, unknown> = {};
		for (const [k, v] of Object.entries(value)) {
			if (!isSerializable(v)) continue;
			const next = toSerializable(v);
			if (next !== undefined) out[k] = next;
		}
		return out;
	}
	return value;
}

let saveTimeout: ReturnType<typeof setTimeout> | null = null;
let lastUserId: string | undefined;
let lastWorkspaceId: string | undefined;
let lastKey: string | null = null;
let pendingLayout: Layout | null = null;

function flushSave() {
	if (!lastUserId || !lastWorkspaceId || !pendingLayout) return;
	if (saveTimeout) {
		clearTimeout(saveTimeout);
		saveTimeout = null;
	}
	const serializable = toSerializable(pendingLayout);
	tryCatch(() =>
		localStorage.setItem(getStorageKey(lastUserId!, lastWorkspaceId!), JSON.stringify(serializable))
	);
}

export function initLayoutPersistence(): () => void {
	let layoutUnsub: (() => void) | undefined;

	const wsUnsub = workspaceGraphStore.subscribe((ws) => {
		const userId = (ws as graph.WorkspaceNode | undefined)?.user?.id;
		const workspaceId = ws?.id;
		const key = userId && workspaceId ? `${userId}:${workspaceId}` : null;

		if (!key) {
			lastKey = null;
			lastUserId = undefined;
			lastWorkspaceId = undefined;
			layoutUnsub?.();
			layoutUnsub = undefined;
			return;
		}

		lastUserId = userId;
		lastWorkspaceId = workspaceId;
		if (key !== lastKey) {
			lastKey = key;
			const raw = localStorage.getItem(getStorageKey(userId!, workspaceId!));
			const [layout, err] = raw ? tryCatch(() => JSON.parse(raw) as Layout) : [null, true];
			layoutStore.set(!err && layout ? layout : createInitialLayout());
		}

		layoutUnsub?.();
		layoutUnsub = layoutStore.subscribe((layout) => {
			const uid = lastUserId;
			const wid = lastWorkspaceId;
			if (!uid || !wid) return;
			pendingLayout = layout;
			if (saveTimeout) clearTimeout(saveTimeout);
			saveTimeout = setTimeout(() => {
				saveTimeout = null;
				flushSave();
			}, SAVE_DEBOUNCE_MS);
		});
	});

	// Flush any pending save when the window closes, onDestroy does not fire on app close
	window.addEventListener('pagehide', flushSave);

	return () => {
		wsUnsub();
		layoutUnsub?.();
		flushSave();
		window.removeEventListener('pagehide', flushSave);
	};
}
