import {
	clearWorkspaceGraphCache,
	initializeWorkspaceGraph,
	workspaceGraphStore
} from '$lib/utils/graph/workspaceGraphStore';
import { loadGitStatus, gitWorkspaceStatusStore } from '$lib/components/views/Git/gitStore';
import { loadMyPermissions, clearMyPermissions } from '$lib/stores/myPermissionsStore';
import { notify } from '$lib/system/Notifications/notificationsStore';
import { AlertType } from '$lib/system/Alert/types';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { tryCatch } from '$lib/utils/tryCatch';
import {
	CheckForLogin,
	CheckForLogout,
	Logout
} from '$lib/bindings/selectDb/internal/system/system';
import { EventsOn } from '$lib/wails/events';
import { stripNullItems, type WorkspaceNode } from '$lib/wails/graph';
import { writable, get } from 'svelte/store';

export const sessionCheckingStore = writable(true);

let checkSessionInterval: ReturnType<typeof setInterval> | undefined;

let lastState: 'loggedin' | 'loggedout' | undefined;

EventsOn('workspaceGraphUpdated', async (g: WorkspaceNode) => {
	stripNullItems(g);
	if (lastState === 'loggedin') {
		workspaceGraphStore.set(g);
	}
});

type WorkspaceRepoChange = {
	action: 'noop' | 'linked' | 'switched' | 'unlinked';
	changed: boolean;
	remoteUrl: string;
	backupPath: string;
	backupRef: string;
};

EventsOn('workspaceRepoChanged', async (info: WorkspaceRepoChange) => {
	if (lastState !== 'loggedin') return;
	await loadGitStatus();

	if (info.action === 'switched') {
		notify({
			type: AlertType.Default,
			message: info.backupPath
				? `This workspace was relinked to a different git repository by an admin. Your previous local content was saved to ${info.backupPath}`
				: 'This workspace was relinked to a different git repository by an admin.',
			duration: 10000,
			copyable: true
		});
	} else if (info.action === 'unlinked') {
		notify({
			type: AlertType.Default,
			message:
				'This workspace is no longer linked to a git repository. Your files were kept locally.',
			duration: 8000
		});
	} else if (info.action === 'linked' && info.changed) {
		notify({
			type: AlertType.Success,
			message: 'This workspace is now linked to its git repository.',
			duration: 5000
		});
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

	// The workspace expects a git repo but git is missing: its files cannot
	// sync. Surface this once so the user understands why content is stale.
	const gitStatus = get(gitWorkspaceStatusStore);
	if (gitStatus && !gitStatus.gitAvailable && gitStatus.configuredRemoteUrl) {
		notify({
			type: AlertType.Default,
			message:
				'This workspace is linked to a Git repository, but Git is not installed. Its files cannot sync until you install Git.',
			duration: 12000,
			copyable: true
		});
	}
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
