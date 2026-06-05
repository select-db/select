import type { generated } from '$lib/wailsjs/go/models';
import { expandItem } from '$lib/components/views/shared/sharedStore';

export async function createFile(commit: generated.MutationCommit) {
	if (commit.operation !== 'insert') return;
	if (commit.table_name !== 'file') return;

	if (!commit.payload.folder_id) return;
	expandItem(commit.payload.folder_id);
}
