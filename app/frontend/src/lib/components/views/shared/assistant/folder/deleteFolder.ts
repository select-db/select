import type * as generated from '$lib/bindings/selectDb/internal/db/generated/models';

export function deleteFolder(commit: generated.MutationCommit) {
	if (commit.operation !== 'delete') return;
	if (commit.table_name !== 'folder') return;

	return;
}
