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
import { loadGitStatus } from '$lib/components/views/Git/gitStore';
import { loadMyPermissions, clearMyPermissions } from '$lib/stores/myPermissionsStore';
import { loadCurrentUser, clearCurrentUser } from '$lib/stores/currentUserStore';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { tryCatch } from '$lib/utils/tryCatch';
import { CheckForLogin, CheckForLogout } from '$lib/bindings/selectDb/internal/system/system';
import { EventsOn } from '$lib/wails/events';
import { stripNullItems, type WorkspaceNode } from '$lib/wails/graph';
import { get, writable } from 'svelte/store';

export const sessionCheckingStore = writable(true);

let checkSessionInterval: ReturnType<typeof setInterval> | undefined;

let lastState: 'loggedin' | 'loggedout' | undefined;

// The watcher debounces, so an update it emitted before a folder closed lands
// after it. Applied blindly it puts the workbench back for a workspace that is
// no longer open, which after a delete is one that no longer exists.
EventsOn('workspaceGraphUpdated', async (g: WorkspaceNode) => {
	stripNullItems(g);
	if (lastState !== 'loggedin') return;

	const folder = get(folderStore);
	if (folder?.status !== WorkspaceStatus.Ready || folder.workspaceId !== g.id) return;

	workspaceGraphStore.set(g);
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
	clearCurrentUser();
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

	// Keyed on the user, not the folder, so they need not wait on the tree walk.
	const permissions = loadMyPermissions();
	const user = loadCurrentUser();

	await reopenLastFolder();

	modalStore.set(null);
	checkSessionInterval = setInterval(() => CheckForLogout(), 500);
	await Promise.all([loadGitStatus(), permissions, user]);
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
