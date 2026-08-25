import { navigateToDatabase } from '$lib/components/views/shared/navigateToDatabase';
import type { Icons } from '$lib/system/Icon/types';
import type * as graph from '$lib/wails/graph';

export const databaseActions = [
	{
		icon: 'edit' as Icons,
		onClick: async (database: graph.DBInstanceNode) => {
			await navigateToDatabase(database);
		}
	}
];
