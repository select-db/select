import { addSchemaTab } from '$lib/components/Layout/layoutStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { loadSchemaIfEmpty } from '$lib/utils/query/loadSchema';
import { get } from 'svelte/store';

export const navigateToSchema = async (databaseId?: string) => {
	const workspace = get(workspaceGraphStore);
	const database = databaseId
		? (workspace?.db_instances ?? []).find(({ id }) => id === databaseId)
		: undefined;

	addSchemaTab(databaseId, database?.name);

	if (!database) return;

	void loadSchemaIfEmpty(database);
};
