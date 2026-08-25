import type * as generated from '$lib/bindings/selectDb/internal/db/generated/models';
import { getTabByNodeId, updateTab } from '$lib/components/Layout/layoutStore';

export function updateFile(commit: generated.MutationCommit) {
	if (commit.operation !== 'update') return;
	if (commit.table_name !== 'file') return;

	const f = commit.payload;

	// Update the tab with the new file data
	const tab = getTabByNodeId(f.id); // TODO should return multiple tabs
	if (!tab) return;
	if (!tab.file) return;

	if (tab.file.isTemp) return;

	updateTab({
		...tab,
		file: {
			...tab.file,
			node: { ...tab.file.node, ...f }
		}
	});
}
