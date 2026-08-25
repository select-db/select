import { get } from 'svelte/store';

import { StageFile, UnstageFile, RevertFile } from '$lib/bindings/selectDb/internal/git/git';
import type * as graph from '$lib/wails/graph';
import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';

import { must, tryCatch } from '$lib/utils/tryCatch';
import { getDbIds, runStatement } from '$lib/utils/query/helpers';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { notifyError } from '$lib/system/Notifications/notificationsStore';

import { AlertType } from '$lib/system/Alert/types';
import { loadGitFileStatus } from '$lib/components/views/Git/gitStore';
import { uriToGitPath } from '$lib/components/views/Git/helpers';
import { notify } from '$lib/system/Notifications/notificationsStore';
import type { Icons } from '$lib/system/Icon/types';

export const getFileActions = (file: graph.FileNode, ctx: 'fs' | 'git' | 'search') => {
	switch (ctx) {
		case 'search':
			return [];
		case 'fs': {
			const isSql = file.name.endsWith('.sql');
			const isSchema = file.name.endsWith('schema.sql');
			return isSql && !isSchema
				? [
						{
							icon: 'play' as Icons,
							onClick: async (file: graph.FileNode) => {
								const dbIds = getDbIds(file);
								if (dbIds.length === 0) {
									notifyError('Select a database before running this file');
									return;
								}

								const content = await must(tryCatch(fs.ReadFile, { uri: file.uri }));
								if (!content) return;

								const folderId = file.folder_id ?? '';

								notify({ type: AlertType.Default, message: `running ${file.name}` });
								for (const dbId of dbIds) {
									await runStatement({
										statement: content,
										dbInstanceId: dbId,
										fileId: file.id,
										folderId
									});
								}
							}
						}
					]
				: [];
		}
		case 'git': {
			const graph = get(workspaceGraphStore);
			if (!graph) return [];

			const gitPath = uriToGitPath(file.uri, graph.id);

			const actions = [];

			// Show "Stage" (plus icon) if file is unstaged or untracked
			if (file.id.startsWith('git::unstaged') || file.id.startsWith('git::untracked')) {
				actions.push({
					icon: 'plus' as Icons,
					onClick: async () => {
						await must(tryCatch(StageFile, { path: gitPath }));
						notify({ type: AlertType.Success, message: `Staged ${file.name}` });
						await loadGitFileStatus();
					}
				});
			}

			// Show "Unstage" (minus icon) if file is staged
			if (file.id.startsWith('git::staged')) {
				actions.push({
					icon: 'minus' as Icons,
					onClick: async () => {
						await must(tryCatch(UnstageFile, { path: gitPath }));
						notify({ type: AlertType.Success, message: `Unstaged ${file.name}` });
						await loadGitFileStatus();
					}
				});
			}

			// Show "Discard changes" (undo icon) only if file is unstaged (not for untracked files)
			if (file.id.startsWith('git::unstaged')) {
				actions.push({
					icon: 'undo' as Icons,
					onClick: async (file: graph.FileNode) => {
						await must(tryCatch(RevertFile, { path: gitPath }));
						notify({ type: AlertType.Success, message: `Reverted changes to ${file.name}` });
						await loadGitFileStatus();
					}
				});
			}

			// Show "Delete" (undo icon) for untracked files
			if (file.id.startsWith('git::untracked')) {
				actions.push({
					icon: 'undo' as Icons,
					onClick: async (file: graph.FileNode) => {
						await must(tryCatch(fs.Delete, { uri: file.uri, recursive: false }));
						await tryCatch(fs.Delete, { uri: file.uri + '.metadata.json', recursive: false });
						notify({ type: AlertType.Success, message: `Deleted ${file.name}` });
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
