import type { generated } from '$lib/wailsjs/go/models';
import { removeTabByUri } from '$lib/components/Layout/layoutStore';

export async function deleteDatabase(commit: generated.MutationCommit) {
	if (commit.operation !== 'delete') return;

	removeTabByUri(commit.object_id);
}
