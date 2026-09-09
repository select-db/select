import {
	clearWorkspaceGraphCache,
	initializeWorkspaceGraph,
	workspaceGraphStore
} from '$lib/utils/graph/workspaceGraphStore';
import { loadGitStatus } from '$lib/components/views/Git/gitStore';
import { loadMyPermissions, clearMyPermissions } from '$lib/stores/myPermissionsStore';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { tryCatch } from '$lib/utils/tryCatch';
import {
	CheckForLogin,
	CheckForLogout,
	Logout
} from '$lib/bindings/selectDb/internal/system/system';
import { EventsOn } from '$lib/wails/events';
import { stripNullItems, type WorkspaceNode } from '$lib/wails/graph';
import { writable } from 'svelte/store';

export const sessionCheckingStore = writable(true);

let checkSessionInterval: ReturnType<typeof setInterval> | undefined;

let lastState: 'loggedin' | 'loggedout' | undefined;

EventsOn('workspaceGraphUpdated', async (g: WorkspaceNode) => {
	stripNullItems(g);
	if (lastState === 'loggedin') {
		workspaceGraphStore.set(g);
	}
});

EventsOn('logout', () => {
	if (lastState === 'loggedout') return;
	lastState = 'loggedout';
	if (checkSessionInterval) {
		clearInterval(checkSessionInterval);
		checkSessionInterval = undefined;
	}
	clearWorkspaceGraphCache();
	clearMyPermissions();
});

EventsOn('login', async () => {
	if (lastState === 'loggedin') return;
	lastState = 'loggedin';

	// Clear cached graph so we fetch for the current server (backend invalidated on switch).
	clearWorkspaceGraphCache();
	clearMyPermissions();

	const [graph, err] = await tryCatch(initializeWorkspaceGraph);
	if (err) {
		lastState = 'loggedout';
		workspaceGraphStore.set(undefined);
		await tryCatch(Logout);
		return;
	}

	// Set store here so layout sees the graph (init's set can be invisible across async boundary)
	if (graph) workspaceGraphStore.set(graph);
	modalStore.set(null);
	checkSessionInterval = setInterval(() => CheckForLogout(), 500);
	await Promise.all([loadGitStatus(), loadMyPermissions()]);
});

export const setupSessionWall = async () => {
	await tryCatch(CheckForLogin);
	sessionCheckingStore.set(false);
};

export const teardownSessionWall = () => {
	if (checkSessionInterval) {
		clearInterval(checkSessionInterval);
		checkSessionInterval = undefined;
	}
};
