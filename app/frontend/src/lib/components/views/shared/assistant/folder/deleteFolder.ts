import type { generated } from '$lib/wailsjs/go/models';

export function deleteFolder(commit: generated.MutationCommit) {
	if (commit.operation !== 'delete') return;
	if (commit.table_name !== 'folder') return;

	return;
}
