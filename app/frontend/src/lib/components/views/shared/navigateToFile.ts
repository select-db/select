import { addTab } from '$lib/components/Layout/layoutStore';
import { setItemSelection } from '$lib/components/views/shared/sharedStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { loadSchemaIfEmpty } from '$lib/utils/query/loadSchema';
import type * as graph from '$lib/wails/graph';
import { get } from 'svelte/store';

export const navigateToFile = async (file: graph.FileNode) => {
	setItemSelection([file.id]);
	addTab(file);

	const workspace = get(workspaceGraphStore);
	const fileDatasource = (workspace?.datasources ?? []).find(
		({ id }) => id === file.datasources?.[0]?.id
	);
	if (fileDatasource) void loadSchemaIfEmpty(fileDatasource);
};
