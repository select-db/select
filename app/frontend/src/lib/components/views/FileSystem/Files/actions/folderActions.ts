import { get } from 'svelte/store';

import { StageAll, UnstageAll, RevertAll } from '$lib/wailsjs/go/git/Git';
import type { graph } from '$lib/wailsjs/go/models';

import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { must, tryCatch } from '$lib/utils/tryCatch';

import { AlertType } from '$lib/system/Alert/types';
import { notify } from '$lib/system/Notifications/notificationsStore';
import type { Icons } from '$lib/system/Icon/types';

import { loadGitFileStatus } from '$lib/components/views/Git/gitStore';

export const getFolderActions = (folder: graph.FolderNode, ctx: 'fs' | 'git' | 'search') => {
	switch (ctx) {
		case 'search':
		case 'fs':
			return [];
		case 'git': {
			const graph = get(workspaceGraphStore);
			if (!graph) return [];

			const actions = [];

			// Special case for "Staged changes" placeholder folder
			if (folder.id === 'git::staged') {
				actions.push({
					icon: 'minus' as Icons,
					onClick: async () => {
						await must(tryCatch(UnstageAll));
						notify({ type: AlertType.Success, message: 'Unstaged all changes' });
						await loadGitFileStatus();
					}
				});
			}

			// Special case for "Unstaged changes" placeholder folder
			if (folder.id === 'git::unstaged') {
				actions.push({
					icon: 'plus' as Icons,
					onClick: async () => {
						await must(tryCatch(StageAll));
						notify({ type: AlertType.Success, message: 'Staged all changes' });
						await loadGitFileStatus();
					}
				});

				actions.push({
					icon: 'undo' as Icons,
					onClick: async () => {
						await must(tryCatch(RevertAll));
						notify({ type: AlertType.Success, message: 'Reverted all changes' });
						await loadGitFileStatus();
					}
				});
			}

			return actions;
		}

		default:
			return [];
	}
};
