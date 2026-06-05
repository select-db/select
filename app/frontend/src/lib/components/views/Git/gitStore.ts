import { writable } from 'svelte/store';
import { GetGitFileStatus, GetGitWorkspaceStatus } from '$lib/wailsjs/go/git/Git';
import type { git } from '$lib/wailsjs/go/models';
import { must, tryCatch } from '$lib/utils/tryCatch';
import { EventsOn } from '$lib/wailsjs/runtime/runtime';

export const gitWorkspaceStatusStore = writable<git.GitWorkspaceStatus | null>(null);
export const gitFileStatusStore = writable<git.GitFileStatus | null>(null);

EventsOn('gitDetailedStatusChanged', () => loadGitStatus());

export const loadGitStatus = async (): Promise<void> => {
	const status = await must(tryCatch(GetGitWorkspaceStatus));
	gitWorkspaceStatusStore.set(status);

	if (status.isGitRepo && status.hasRemote) {
		await loadGitFileStatus();
	} else {
		gitFileStatusStore.set(null);
	}
};

export const loadGitFileStatus = async (): Promise<void> => {
	const status = await must(tryCatch(GetGitFileStatus));
	gitFileStatusStore.set(status);
};

