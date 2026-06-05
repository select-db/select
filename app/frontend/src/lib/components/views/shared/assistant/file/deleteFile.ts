import type { generated } from '$lib/wailsjs/go/models';
import {
	removeFromItemSelection,
	renamingItemIdStore
} from '$lib/components/views/shared/sharedStore';
import { removeTabByUri } from '$lib/components/Layout/layoutStore';

export async function deleteFile(commit: generated.MutationCommit) {
	if (commit.operation !== 'delete') return;
	if (commit.table_name !== 'file') return;

	removeTabByUri(commit.object_id);
	removeFromItemSelection(commit.object_id);
	renamingItemIdStore.update((id) => (id === commit.object_id ? null : id));
}
