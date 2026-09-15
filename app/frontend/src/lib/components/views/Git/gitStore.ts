import { get, writable } from 'svelte/store';
import { GetGitFileStatus, GetGitWorkspaceStatus } from '$lib/bindings/selectDb/internal/git/git';
import type * as git from '$lib/bindings/selectDb/internal/git/models';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import { tryCatch } from '$lib/utils/tryCatch';
import { EventsOn } from '$lib/wails/events';

export const gitWorkspaceStatusStore = writable<git.GitWorkspaceStatus | null>(null);
export const gitFileStatusStore = writable<git.GitFileStatus | null>(null);

EventsOn('gitDetailedStatusChanged', () => loadGitStatus());

// The store holds undefined when no folder is open, never null.
const folderIsOpen = () => get(workspaceGraphStore) !== undefined;

/**
 * Git belongs to the open folder, so a folder closing under a call in flight
 * fails it. That is not a failure to put in front of the user.
 */
const report = (message: string) => {
	if (folderIsOpen()) notifyError(message);
};

export const loadGitStatus = async (): Promise<void> => {
	if (!folderIsOpen()) {
		gitWorkspaceStatusStore.set(null);
		gitFileStatusStore.set(null);
		return;
	}

	const [status, err] = await tryCatch(GetGitWorkspaceStatus);
	if (err) {
		report(err.message);
		return;
	}
	gitWorkspaceStatusStore.set(status);

	// A repository is enough. Staging and committing need no origin, and gating
	// on one left a git init with no remote showing an empty panel.
	if (status?.isGitRepo) {
		await loadGitFileStatus();
	} else {
		gitFileStatusStore.set(null);
	}
};

export const loadGitFileStatus = async (): Promise<void> => {
	if (!folderIsOpen()) {
		gitFileStatusStore.set(null);
		return;
	}

	const [status, err] = await tryCatch(GetGitFileStatus);
	if (err) {
		report(err.message);
		return;
	}
	gitFileStatusStore.set(status);
};
