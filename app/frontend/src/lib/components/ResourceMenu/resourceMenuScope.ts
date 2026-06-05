import type { graph } from '$lib/wailsjs/go/models';
import type { ResourceMenuOption, ResourceSearchScope } from './types';

/** Infix between DB instance id and schema name in graph node ids (see backend schema.go). */
const SCHEMA_ID_INFIX = ':schema:';

export function parseDbInstanceIdFromSchemaId(schemaId: string): string | null {
	const i = schemaId.indexOf(SCHEMA_ID_INFIX);
	if (i === -1) return null;
	return schemaId.slice(0, i);
}

/**
 * Maps a db_item id to its schema root id using the known schema ids from the workspace.
 * Uses the longest matching schema id prefix so schema names may contain ':'.
 */
function schemaIdForCatalogItem(
	itemId: string,
	knownSchemaIds: ReadonlySet<string>
): string | null {
	if (knownSchemaIds.has(itemId)) return itemId;

	for (const sid of knownSchemaIds) {
		if (itemId.startsWith(`${sid}:`)) return sid;
	}

	return null;
}

export function resourceOptionInSearchScope(
	opt: ResourceMenuOption,
	scope: ResourceSearchScope
): boolean {
	if (opt.type === 'file' || opt.type === 'temp_file') return true;

	if (opt.type === 'db_instance') {
		return scope.enabledDbIds.has((opt.node as graph.DBInstanceNode).id);
	}

	if (opt.type === 'db_item') {
		if (opt.node.type === 'table' && opt.node.id === 'AppAccount') {
			console.log(opt);
		}
		const schemaId = schemaIdForCatalogItem(
			(opt.node as graph.DBInstanceItemNode).id,
			scope.knownSchemaIds
		);
		return schemaId != null && scope.enabledSchemaIds.has(schemaId);
	}

	return true;
}
