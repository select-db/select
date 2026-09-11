import {
	clearWorkspaceGraphCache,
	workspaceGraphStore
} from '$lib/utils/graph/workspaceGraphStore';
import {
	clearFolderState,
	folderStore,
	onFolderClosed,
	reopenLastFolder
} from '$lib/components/PageFolder/folderStore';
import { WorkspaceStatus } from '$lib/bindings/selectDb/internal/workspace/models';
import { loadMyPermissions, clearMyPermissions } from '$lib/stores/myPermissionsStore';
import { loadCurrentUser, clearCurrentUser } from '$lib/stores/currentUserStore';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { tryCatch } from '$lib/utils/tryCatch';
import { CheckForLogin, CheckForLogout } from '$lib/bindings/selectDb/internal/system/system';
import { EventsOn } from '$lib/wails/events';
import { stripNullItems, type WorkspaceNode } from '$lib/wails/graph';
import { get, writable } from 'svelte/store';

export const sessionCheckingStore = writable(true);

let logoutPollInterval: ReturnType<typeof setInterval> | undefined;

let sessionState: 'loggedin' | 'loggedout' | undefined;

// The watcher debounces, so an update emitted before a folder closed can land
// after it. Applied blindly it restores the workbench for a workspace that is
// no longer open.
EventsOn('workspaceGraphUpdated', async (updatedGraph: WorkspaceNode) => {
	if (sessionState !== 'loggedin') return;

	const folder = get(folderStore);
	if (folder?.status !== WorkspaceStatus.Ready || folder.workspaceId !== updatedGraph.id) return;

	stripNullItems(updatedGraph);
	workspaceGraphStore.set(updatedGraph);
});

EventsOn('logout', () => {
	if (sessionState === 'loggedout') return;
	sessionState = 'loggedout';
	if (logoutPollInterval) {
		clearInterval(logoutPollInterval);
		logoutPollInterval = undefined;
	}
	clearWorkspaceGraphCache();
	clearMyPermissions();
	clearCurrentUser();
	clearFolderState();
});

EventsOn('workspaceClosed', () => {
	if (sessionState !== 'loggedin') return;
	void onFolderClosed();
});

EventsOn('login', async () => {
	if (sessionState === 'loggedin') return;
	sessionState = 'loggedin';

	// Drop the cached graph so the next load fetches from the server just signed in to.
	clearWorkspaceGraphCache();
	clearMyPermissions();

	// Started before the folder reopens, since neither call depends on the tree walk.
	const permissions = loadMyPermissions();
	const user = loadCurrentUser();

	await reopenLastFolder();

	modalStore.set(null);
	logoutPollInterval = setInterval(() => CheckForLogout(), 500);
	await Promise.all([permissions, user]);
});

export const setupSessionWall = async () => {
	await tryCatch(CheckForLogin);
	sessionCheckingStore.set(false);
};

export const teardownSessionWall = () => {
	if (logoutPollInterval) {
		clearInterval(logoutPollInterval);
		logoutPollInterval = undefined;
	}
};
