import { addTab } from '$lib/components/Layout/layoutStore';
import { setItemSelection } from '$lib/components/views/shared/sharedStore';
import { loadSchema } from '$lib/utils/query/loadSchema';
import type { graph } from '$lib/wailsjs/go/models';

export const navigateToDatabase = async (database: graph.DBInstanceNode) => {
	setItemSelection([database.id]);
	addTab(database);

	const shouldLoadSchema =
		'children' in database && (!database.children || database.children.length === 0);

	if (shouldLoadSchema) loadSchema({ database, silent: true });
};
