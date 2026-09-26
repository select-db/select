import { addTab } from '$lib/components/Layout/layoutStore';
import { setItemSelection } from '$lib/components/views/shared/sharedStore';
import { loadSchemaIfEmpty } from '$lib/utils/query/loadSchema';
import type * as graph from '$lib/wails/graph';

export const navigateToDatasource = async (datasource: graph.DatasourceNode) => {
	setItemSelection([datasource.id]);
	addTab(datasource);

	void loadSchemaIfEmpty(datasource);
};
