import { navigateToDatabase } from '$lib/components/views/shared/navigateToDatabase';
import type { Icons } from '$lib/system/Icon/types';
import type { graph } from '$lib/wailsjs/go/models';

export const databaseActions = [
	{
		icon: 'edit' as Icons,
		onClick: async (database: graph.DBInstanceNode) => {
			await navigateToDatabase(database);
		}
	}
];
