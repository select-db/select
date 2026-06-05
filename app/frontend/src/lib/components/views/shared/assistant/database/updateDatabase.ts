import type { generated } from '$lib/wailsjs/go/models';
import { renamingItemIdStore } from '$lib/components/views/shared/sharedStore';
import { getTabByNodeId, updateTab } from '$lib/components/Layout/layoutStore';

export function updateDatabase(commit: generated.MutationCommit) {
	if (commit.operation !== 'update') return;
	if (commit.table_name !== 'db_instance') return;

	const db = commit.payload;

	renamingItemIdStore.set(null);

	// Update the tab with the new database data
	const tab = getTabByNodeId(db.id);
	if (tab && tab.database) {
		updateTab({
			...tab,
			database: {
				...tab.database,
				node: { ...tab.database.node, ...db }
			}
		});
	}
}
