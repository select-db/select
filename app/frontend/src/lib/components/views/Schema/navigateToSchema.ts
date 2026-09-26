import { addSchemaTab } from '$lib/components/Layout/layoutStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { loadSchemaIfEmpty } from '$lib/utils/query/loadSchema';
import { get } from 'svelte/store';

export const navigateToSchema = async (dbInstanceId?: string) => {
	const workspace = get(workspaceGraphStore);
	const database = dbInstanceId
		? (workspace?.db_instances ?? []).find(({ id }) => id === dbInstanceId)
		: undefined;

	addSchemaTab(dbInstanceId, database?.name);

	if (!database) return;

	void loadSchemaIfEmpty(database);
};
