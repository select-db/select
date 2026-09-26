import { addSchemaTab } from '$lib/components/Layout/layoutStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { loadSchemaIfEmpty } from '$lib/utils/query/loadSchema';
import { get } from 'svelte/store';

export const navigateToSchema = async (datasourceId?: string) => {
	const workspace = get(workspaceGraphStore);
	const datasource = datasourceId
		? (workspace?.datasources ?? []).find(({ id }) => id === datasourceId)
		: undefined;

	addSchemaTab(datasourceId, datasource?.name);

	if (!datasource) return;

	void loadSchemaIfEmpty(datasource);
};
