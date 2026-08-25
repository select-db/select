import { writable } from 'svelte/store';
import { GetGitFileStatus, GetGitWorkspaceStatus } from '$lib/bindings/selectDb/internal/git/git';
import type * as git from '$lib/bindings/selectDb/internal/git/models';
import { must, tryCatch } from '$lib/utils/tryCatch';
import { EventsOn } from '$lib/wails/events';

export const gitWorkspaceStatusStore = writable<git.GitWorkspaceStatus | null>(null);
export const gitFileStatusStore = writable<git.GitFileStatus | null>(null);

EventsOn('gitDetailedStatusChanged', () => loadGitStatus());

export const loadGitStatus = async (): Promise<void> => {
	const status = await must(tryCatch(GetGitWorkspaceStatus));
	gitWorkspaceStatusStore.set(status);

	if (status?.isGitRepo && status.hasRemote) {
		await loadGitFileStatus();
	} else {
		gitFileStatusStore.set(null);
	}
};

export const loadGitFileStatus = async (): Promise<void> => {
	const status = await must(tryCatch(GetGitFileStatus));
	gitFileStatusStore.set(status);
};

