import { addTab } from '$lib/components/Layout/layoutStore';
import { setItemSelection } from '$lib/components/views/shared/sharedStore';
import { loadSchemaIfEmpty } from '$lib/utils/query/loadSchema';
import type * as graph from '$lib/wails/graph';

export const navigateToDatabase = async (database: graph.DBInstanceNode) => {
	setItemSelection([database.id]);
	addTab(database);

	void loadSchemaIfEmpty(database);
};
