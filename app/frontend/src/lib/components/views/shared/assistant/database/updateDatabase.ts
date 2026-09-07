import type * as generated from '$lib/bindings/selectDb/internal/db/generated/models';
import { renamingItemIdStore } from '$lib/components/views/shared/sharedStore';
import { getTabByNodeId, updateTab } from '$lib/components/Layout/layoutStore';

export function updateDatabase(commit: generated.MutationCommit) {
	if (commit.operation !== 'update') return;
	if (commit.table_name !== 'db_instance') return;

	const db = commit.payload;

	renamingItemIdStore.set(null);

	// Update the tab with the new database data.
	//
	// Its URI as well as its node: a database is a directory named after
	// itself, so a rename moves it, and a tab left holding the old path is a
	// tab the next open will not recognise as this database — it opens a
	// second one beside it.
	const tab = getTabByNodeId(db.id);
	if (tab && tab.database) {
		updateTab({
			...tab,
			uri: db.uri ?? tab.uri,
			database: {
				...tab.database,
				node: { ...tab.database.node, ...db }
			}
		});
	}
}
