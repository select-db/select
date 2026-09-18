import type * as generated from '$lib/bindings/selectDb/internal/db/generated/models';
import { removeTabByUri } from '$lib/components/Layout/layoutStore';

export async function deleteDatabase(commit: generated.MutationCommit) {
	if (commit.operation !== 'delete') return;

	removeTabByUri(commit.object_id);
}
