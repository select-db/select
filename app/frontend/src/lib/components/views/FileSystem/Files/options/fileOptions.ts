import { get } from 'svelte/store';

import { GetOSPathFromURI } from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
import { RevealInExplorer } from '$lib/bindings/selectDb/internal/system/system';
import { StageFile, UnstageFile, RevertFile } from '$lib/bindings/selectDb/internal/git/git';
import type * as git from '$lib/bindings/selectDb/internal/git/models';
import type * as graph from '$lib/wails/graph';
import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';

import { must, tryCatch } from '$lib/utils/tryCatch';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';

import { AlertType } from '$lib/system/Alert/types';
import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { notify } from '$lib/system/Notifications/notificationsStore';

import { addToItemSelection } from '$lib/components/views/shared/sharedStore';
import { navigateToFile } from '$lib/components/views/shared/navigateToFile';
import { renamingItemIdStore } from '$lib/components/views/shared/sharedStore';
import { loadGitFileStatus, gitFileStatusStore } from '$lib/components/views/Git/gitStore';
import { uriToGitPath } from '$lib/components/views/Git/helpers';

export const uriToRelativePath = (uri: string): string => {
	const parts = uri.split('/');
	return parts.slice(4).join('/');
};

export const getExplorerLabel = () => {
	const platform = navigator.userAgent.toLowerCase();
	if (platform.includes('mac')) return 'Reveal in Finder';
	if (platform.includes('win')) return 'Reveal in File Explorer';
	return 'Reveal in File Manager';
};

export const fileSystemOptions = [
	{
		label: '',
		divider: true
	},
	{
		label: 'Copy path',
		action: async (onClose, { uri }: graph.FileNode) => {
			const systemPath = await GetOSPathFromURI(uri);
			await navigator.clipboard.writeText(systemPath);
			notify({
				type: AlertType.Success,
				message: 'Copied to clipboard'
			});
			onClose();
		}
	},
	{
		label: 'Copy relative path',
		action: async (onClose, { uri }: graph.FileNode) => {
			const relativePath = uriToRelativePath(uri);
			await navigator.clipboard.writeText(relativePath);
			notify({
				type: AlertType.Success,
				message: 'Copied to clipboard'
			});
			onClose();
		}
	},
	{
		label: getExplorerLabel(),
		action: async (onClose, { uri }: graph.FileNode) => {
			await RevealInExplorer(uri);
			onClose();
		}
	}
] satisfies ContextMenuOption[];

const fsFileOptions = [
	{
		label: 'Open',
		runOnDoubleClick: true,
		action: async (onClose, file: graph.FileNode) => {
			await navigateToFile(file);
			onClose();
		}
	},
	{
		label: 'Rename...',
		action: (onClose, { id }: graph.FileNode) => {
			renamingItemIdStore.set(id);
			addToItemSelection(id);
			onClose?.();
		}
	},
	...fileSystemOptions,
	{
		label: '',
		divider: true
	},
	{
		label: 'Delete',
		action: async (onClose, { uri }: graph.FileNode) => {
			await must(tryCatch(fs.Delete, { uri, recursive: false }));
			await must(tryCatch(fs.Delete, { uri: uri + '.metadata.json', recursive: false }));
			onClose();
		}
	}
] satisfies ContextMenuOption[];

const getGitFileOptions = (file: graph.FileNode): ContextMenuOption[] => {
	const graph = get(workspaceGraphStore);
	if (!graph) return [];

	const status: git.GitFileStatus | null = get(gitFileStatusStore);
	if (!status) return [];

	const gitPath = uriToGitPath(file.uri, graph.id);

	const options: ContextMenuOption[] = [];

	// Show "Stage" if file is unstaged or untracked
	if (file.id.startsWith('git::unstaged') || file.id.startsWith('git::untracked')) {
		options.push({
			label: 'Stage',
			action: async (onClose) => {
				await must(tryCatch(StageFile, { path: gitPath }));
				await loadGitFileStatus();
				onClose?.();
			}
		});
	}

	// Show "Unstage" if file is staged
	if (file.id.startsWith('git::staged')) {
		options.push({
			label: 'Unstage',
			action: async (onClose) => {
				await must(tryCatch(UnstageFile, { path: gitPath }));
				await loadGitFileStatus();
				onClose?.();
			}
		});
	}

	// Show "Discard changes" only if file is unstaged (not for untracked files)
	if (file.id.startsWith('git::unstaged')) {
		options.push({
			label: 'Discard changes',
			action: async (onClose) => {
				await must(tryCatch(RevertFile, { path: gitPath }));
				notify({ type: AlertType.Success, message: `Discarded changes to ${file.name}` });
				await loadGitFileStatus();
				onClose?.();
			}
		});
	}

	// Show "Delete" for untracked files
	if (file.id.startsWith('git::untracked')) {
		options.push({
			label: 'Delete',
			action: async (onClose, { uri }: graph.FileNode) => {
				await must(tryCatch(fs.Delete, { uri, recursive: false }));
				await tryCatch(fs.Delete, { uri: uri + '.metadata.json', recursive: false });
				notify({ type: AlertType.Success, message: 'File deleted' });
				await loadGitFileStatus();
				onClose();
			}
		});
	}

	return options;
};

export const fileOptions = fsFileOptions;

export const getFileOptions = (
	file: graph.FileNode,
	ctx: 'fs' | 'git' | 'search'
): ContextMenuOption[] => {
	switch (ctx) {
		case 'fs':
			return fsFileOptions;
		case 'git':
			return getGitFileOptions(file);
		case 'search':
			return [];
		default:
			return fsFileOptions;
	}
};
