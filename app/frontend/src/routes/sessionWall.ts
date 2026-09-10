import {
	clearWorkspaceGraphCache,
	workspaceGraphStore
} from '$lib/utils/graph/workspaceGraphStore';
import {
	clearFolderState,
	onFolderClosed,
	reopenLastFolder
} from '$lib/components/PageFolder/folderStore';
import { loadGitStatus } from '$lib/components/views/Git/gitStore';
import { loadMyPermissions, clearMyPermissions } from '$lib/stores/myPermissionsStore';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { tryCatch } from '$lib/utils/tryCatch';
import { CheckForLogin, CheckForLogout } from '$lib/bindings/selectDb/internal/system/system';
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
	clearFolderState();
});

EventsOn('workspaceClosed', () => {
	if (lastState !== 'loggedin') return;
	void onFolderClosed();
});

EventsOn('login', async () => {
	if (lastState === 'loggedin') return;
	lastState = 'loggedin';

	// Clear cached graph so we fetch for the current server (backend invalidated on switch).
	clearWorkspaceGraphCache();
	clearMyPermissions();

	// Keyed on the user, not the folder, so it need not wait on the tree walk.
	const permissions = loadMyPermissions();

	await reopenLastFolder();

	modalStore.set(null);
	checkSessionInterval = setInterval(() => CheckForLogout(), 500);
	await Promise.all([loadGitStatus(), permissions]);
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
