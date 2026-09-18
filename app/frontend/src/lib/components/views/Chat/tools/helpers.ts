import { get } from 'svelte/store';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { GetDBInstanceNodeByID } from '$lib/wails/graph';
import { QuerySchema } from '$lib/bindings/selectDb/internal/db_client/dbclient';
import { tryCatch } from '$lib/utils/tryCatch';
import { toToolError } from './context';
import type * as graph from '$lib/wails/graph';
import {
	dbInstanceToContextDb,
	type ChatContextDatabase
} from '$lib/components/Layout/layoutStore';

export type LoadDatabaseResult =
	| { db: ChatContextDatabase; node: graph.DBInstanceNode; error?: undefined }
	| { db?: undefined; node?: undefined; error: string };

/**
 * Load database by id from the workspace graph. If schema is not loaded yet, fetches it.
 * Returns the db (and the graph node for e.g. reading .children) or an error.
 */
export async function loadDatabase(databaseInstanceId: string): Promise<LoadDatabaseResult> {
	const graph = get(workspaceGraphStore);

	if (!graph) {
		return {
			error: 'Workspace graph not found.'
		};
	}

	let node = graph.db_instances.find((d) => d.id === databaseInstanceId);

	if (!node) {
		return {
			error: `Database not found: ${databaseInstanceId}. Use an id from the context block databases[].id.`
		};
	}

	// Load schema if not yet in graph
	if (!node.children || node.children.length === 0) {
		const [, err] = await tryCatch(QuerySchema, {
			DatabaseInstanceID: databaseInstanceId,
			NoCache: false
		});

		if (err) {
			return { error: toToolError(err) };
		}

		const [updatedNode, fetchErr] = await tryCatch(GetDBInstanceNodeByID, databaseInstanceId);
		if (fetchErr || !updatedNode) {
			return {
				error: fetchErr ? toToolError(fetchErr) : 'Failed to load database node after schema fetch'
			};
		}

		node = updatedNode;
	}

	return { db: dbInstanceToContextDb(node), node };
}
