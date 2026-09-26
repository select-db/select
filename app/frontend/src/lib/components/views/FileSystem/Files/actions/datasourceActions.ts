import { navigateToDatasource } from '$lib/components/views/shared/navigateToDatasource';
import type { Icons } from '$lib/system/Icon/types';
import type * as graph from '$lib/wails/graph';

export const datasourceActions = [
	{
		icon: 'edit' as Icons,
		onClick: async (datasource: graph.DatasourceNode) => {
			await navigateToDatasource(datasource);
		}
	}
];
