import type { generated } from '$lib/wailsjs/go/models';
import { expandItem } from '$lib/components/views/shared/sharedStore';

export function createFolder(commit: generated.MutationCommit) {
	if (commit.operation !== 'insert') return;
	if (commit.table_name !== 'folder') return;

	const f = commit.payload;

	if (!f.folder_id) return;
	expandItem(f.folder_id);
}
