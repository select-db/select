import { get } from 'svelte/store';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { GetDatasourceNodeByID } from '$lib/wails/graph';
import { QuerySchema } from '$lib/bindings/selectDb/internal/db_client/dbclient';
import { tryCatch } from '$lib/utils/tryCatch';
import { toToolError } from './context';
import type * as graph from '$lib/wails/graph';
import {
	datasourceToContext,
	type ChatContextDatasource
} from '$lib/components/Layout/layoutStore';

export type LoadDatasourceResult =
	| { db: ChatContextDatasource; node: graph.DatasourceNode; error?: undefined }
	| { db?: undefined; node?: undefined; error: string };

/**
 * Load database by id from the workspace graph. If schema is not loaded yet, fetches it.
 * Returns the db (and the graph node for e.g. reading .children) or an error.
 */
export async function loadDatasource(datasourceId: string): Promise<LoadDatasourceResult> {
	const graph = get(workspaceGraphStore);

	if (!graph) {
		return {
			error: 'Workspace graph not found.'
		};
	}

	let node = graph.datasources.find((d) => d.id === datasourceId);

	if (!node) {
		return {
			error: `Datasource not found: ${datasourceId}. Use an id from the context block datasources[].id.`
		};
	}

	// Load schema if not yet in graph
	if (!node.children || node.children.length === 0) {
		const [, err] = await tryCatch(QuerySchema, {
			DatasourceID: datasourceId,
			NoCache: false
		});

		if (err) {
			return { error: toToolError(err) };
		}

		const [updatedNode, fetchErr] = await tryCatch(GetDatasourceNodeByID, datasourceId);
		if (fetchErr || !updatedNode) {
			return {
				error: fetchErr ? toToolError(fetchErr) : 'Failed to load database node after schema fetch'
			};
		}

		node = updatedNode;
	}

	return { db: datasourceToContext(node), node };
}
