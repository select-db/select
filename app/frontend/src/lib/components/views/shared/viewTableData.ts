import { get } from 'svelte/store';

import { addTempFileTab } from '$lib/components/Layout/layoutStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { tryCatch } from '$lib/utils/tryCatch';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import * as dbc from '$lib/bindings/selectDb/internal/db_client/dbclient';
import * as db_client from '$lib/bindings/selectDb/internal/db_client/models';
import type * as graph from '$lib/wails/graph';

/**
 * Node types the "View data" action applies to. Views sit here with tables: the
 * generated statement is a plain SELECT against the relation's name, which a
 * view answers as readily as a table.
 */
const PREVIEWABLE_TYPES = new Set(['table', 'view']);

export const isPreviewableDbItem = (item: graph.DBInstanceItemNode): boolean =>
	PREVIEWABLE_TYPES.has(item.type);

type TableLocation = {
	databaseId: string;
	schema: string;
	folderId: string;
};

/**
 * Locates the database instance and schema owning a catalog item.
 *
 * The tree is db_instance → schema → group ("Tables", "Views", ...) → item, so
 * the item is a direct child of one of the schema's groups. Walking the graph
 * rather than parsing node ids keeps this correct for names containing ':'.
 */
export const findDbItemLocation = (itemId: string): TableLocation | null => {
	const workspace = get(workspaceGraphStore);

	for (const database of (workspace?.db_instances ?? [])) {
		for (const schema of database.children) {
			if (schema.type !== 'schema') continue;

			for (const group of schema.children) {
				if (!group.children.some((child) => child.id === itemId)) continue;

				return {
					databaseId: database.id,
					schema: schema.name,
					folderId: database.folder_id ?? ''
				};
			}
		}
	}

	return null;
};

/**
 * Opens a temp SQL tab holding a preview SELECT for the given table, bound to
 * the owning database and run as soon as the tab mounts.
 */
export const viewTableData = async (item: graph.DBInstanceItemNode) => {
	const location = findDbItemLocation(item.id);
	if (!location) {
		notifyError(`Could not resolve the database of ${item.name}`);
		return;
	}

	const params = db_client.GenerateSelectSQLParams.createFrom({
		databaseId: location.databaseId,
		schema: location.schema,
		table: item.name,
		// 0 lets the backend apply its default preview limit.
		limit: 0
	});

	const [result, err] = await tryCatch(dbc.GenerateSelectSQL, params);
	if (err) {
		notifyError(err.message);
		return;
	}
	if (!result.sql) return;

	addTempFileTab(
		{
			content: result.sql,
			name: `[${item.name}].sql`,
			dbInstanceId: location.databaseId,
			folderId: location.folderId,
			runOnOpen: true
		},
		false
	);
};
