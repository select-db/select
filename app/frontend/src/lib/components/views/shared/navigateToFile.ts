import { addTab } from '$lib/components/Layout/layoutStore';
import { setItemSelection } from '$lib/components/views/shared/sharedStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { loadSchema } from '$lib/utils/query/loadSchema';
import type * as graph from '$lib/wails/graph';
import { get } from 'svelte/store';

export const navigateToFile = async (file: graph.FileNode) => {
	setItemSelection([file.id]);
	addTab(file);

	const workspace = get(workspaceGraphStore);
	const fileDatabase = (workspace?.db_instances ?? []).find(
		({ id }) => id === file.databases?.[0]?.id
	);
	const shouldLoadSchema = fileDatabase && fileDatabase.children.length === 0;

	if (shouldLoadSchema) loadSchema({ database: fileDatabase });
};
