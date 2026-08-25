import type * as graph from '$lib/wails/graph';
import type { SelectOptionGroup } from '$lib/system/Select/Select.types';

type DBInstanceItemNode = graph.DBInstanceItemNode;

function getMetadataSql(node: DBInstanceItemNode): string | undefined {
	const meta = node.metadata;
	if (!meta || typeof meta !== 'object' || Array.isArray(meta)) return undefined;
	const m = meta as { sql?: string };
	return m.sql;
}

/**
 * Returns DDL for a single table/view in the given database: table DDL + indexes + triggers
 * (same order as backend buildSchemaSQLFromMetadata for one table).
 * @param graph - Workspace graph (from workspaceGraphStore)
 * @param databaseId - DB instance id
 * @param schemaTable - "schemaName:tableName"
 */
export function getTableDDL(
	graph: graph.WorkspaceNode | undefined,
	databaseId: string,
	schemaTable: string
): string | null {
	if (!graph?.db_instances) return null;
	const sep = schemaTable.indexOf(':');
	if (sep <= 0) return null;
	const schemaName = schemaTable.slice(0, sep);
	const tableName = schemaTable.slice(sep + 1);
	if (!schemaName || !tableName) return null;

	const dbNode = graph.db_instances.find((d) => d.id === databaseId);
	if (!dbNode?.children) return null;

	const schemaNode = dbNode.children.find(
		(c) => c.type === 'schema' && (c.name ?? '') === schemaName
	);
	if (!schemaNode?.children) return null;

	const tablesGroup = schemaNode.children.find((c) => c.type === 'tables');
	const viewsGroup = schemaNode.children.find((c) => c.type === 'views');
	const tableNode =
		(tablesGroup?.children ?? []).find((c) => (c.name ?? '') === tableName) ??
		(viewsGroup?.children ?? []).find((c) => (c.name ?? '') === tableName);
	if (!tableNode) return null;

	const parts: string[] = [];
	const tableDdl = getMetadataSql(tableNode);
	if (tableDdl?.trim()) parts.push(tableDdl.trim());

	const indexesGroup = tableNode.children.find((c) => c.type === 'indexes');
	for (const idx of (indexesGroup?.children ?? [])) {
		const ddl = getMetadataSql(idx);
		if (ddl?.trim()) parts.push(ddl.trim());
	}

	const triggersGroup = tableNode.children.find((c) => c.type === 'triggers');
	for (const tr of (triggersGroup?.children ?? [])) {
		const ddl = getMetadataSql(tr);
		if (ddl?.trim()) parts.push(ddl.trim());
	}

	if (parts.length === 0) return null;
	return parts.join('\n\n');
}

/**
 * Returns option groups for a schema/table picker: schemas as groups, tables/views as options.
 * Option value format: "schemaName:tableName".
 */
export function getSchemaTableOptionGroups(
	graph: graph.WorkspaceNode | undefined,
	databaseId: string
): SelectOptionGroup[] {
	if (!graph?.db_instances) return [];
	const dbNode = graph.db_instances.find((d) => d.id === databaseId);
	if (!dbNode?.children) return [];

	const schemaNodes = dbNode.children.filter((c) => c.type === 'schema');
	const groups: SelectOptionGroup[] = [];

	for (const schemaNode of schemaNodes) {
		const schemaName = schemaNode.name ?? '';
		const tablesGroup = schemaNode.children.find((c) => c.type === 'tables');
		const viewsGroup = schemaNode.children.find((c) => c.type === 'views');
		const tableOptions = (tablesGroup?.children ?? []).map((c) => ({
			value: `${schemaName}:${c.name ?? ''}`,
			label: c.name ?? ''
		}));
		const viewOptions = (viewsGroup?.children ?? []).map((c) => ({
			value: `${schemaName}:${c.name ?? ''}`,
			label: `${c.name ?? ''} (view)`
		}));
		const options = [...tableOptions, ...viewOptions];
		if (options.length > 0) {
			groups.push({ label: schemaName, options });
		}
	}

	return groups;
}
