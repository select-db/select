import { get } from 'svelte/store';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { setItemSelection } from '$lib/components/views/shared/sharedStore';
import { addDiffTab } from '$lib/components/Layout/layoutStore';
import { uriToGitPath } from '$lib/components/views/Git/helpers';
import { getFileLanguage } from '$lib/components/views/File/Editor/utils/getFileLanguage';
import { GetFileDiffContent } from '$lib/wailsjs/go/git/Git';
import { tryCatch } from '$lib/utils/tryCatch';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import type { graph } from '$lib/wailsjs/go/models';

/**
 * Opens a git file in a diff tab (HEAD vs working tree). Use for files in the Git panel.
 */
export async function navigateToGitFile(file: graph.FileNode) {
	setItemSelection([file.id]);

	const workspace = get(workspaceGraphStore);
	if (!workspace?.id) {
		notifyError('No workspace');
		return;
	}

	const path = uriToGitPath(file.uri, workspace.id);
	const [result, err] = await tryCatch(GetFileDiffContent, { path });

	if (err) {
		notifyError(`Could not load diff: ${err}`);
		return;
	}

	addDiffTab({
		panels: [
			{ content: result.leftContent ?? '', meta: { label: 'HEAD' } },
			{ content: result.rightContent ?? '', meta: { label: 'Working tree' } }
		],
		language: getFileLanguage(file.name),
		readOnly: true,
		viewMode: 'side-by-side',
		context: 'git',
		meta: { workspaceId: workspace.id, path },
		title: file.name,
		split: false
	});
}
