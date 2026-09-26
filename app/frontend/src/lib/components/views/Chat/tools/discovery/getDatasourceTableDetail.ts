import { toolDefinition } from '$lib/components/views/Chat/core/chat/tool-definition';
import { z } from 'zod';
import { loadDatasource } from '../helpers';

const inputSchema = z.object({
	datasourceId: z.string().describe('Datasource ID from the context block datasources[].id'),
	schemaId: z.string().describe('Schema ID from get_datasource_schemas(...).schemas[].id'),
	tableName: z
		.string()
		.describe('Table or view name from get_datasource_schemas(...).schemas[].tables[]')
});

export const getDatasourceTableDetailDef = toolDefinition({
	name: 'get_datasource_table_detail',
	description: `Returns the DDL for a single table or view. Call after get_datasource_schemas when you need the full definition (columns, types, constraints) to write or validate queries. If error is returned, surface it to the user and do not proceed.`,
	inputSchema,
	outputSchema: z.object({
		datasourceId: z.string(),
		schemaId: z.string(),
		tableName: z.string(),
		ddl: z.string().describe('CREATE TABLE/view statement when available'),
		error: z.string().optional()
	})
});

type ImplArgs = z.infer<typeof inputSchema>;

async function getDatasourceTableDetailImpl(args: unknown) {
	const { datasourceId, schemaId, tableName } = args as ImplArgs;

	const { error, db, node } = await loadDatasource(datasourceId);

	if (error) {
		return {
			datasourceId,
			schemaId,
			tableName: '',
			ddl: '',
			error
		};
	}

	const schemaNode = node!.children.find((c) => c.type === 'schema' && c.id === schemaId);

	if (!schemaNode) {
		return {
			datasourceId: db!.id,
			schemaId,
			tableName: '',
			ddl: '',
			error: `Schema not found: ${schemaId}. Use an id from get_datasource_schemas(...).schemas[].id.`
		};
	}

	const tablesGroup = schemaNode.children.find((c) => c.type === 'tables');
	const viewsGroup = schemaNode.children.find((c) => c.type === 'views');
	const tableNodeFromTables = (tablesGroup?.children ?? []).find((c) => c.name === tableName);
	const tableNodeFromViews = (viewsGroup?.children ?? []).find((c) => c.name === tableName);
	const tableNode = tableNodeFromTables ?? tableNodeFromViews;

	if (!tableNode) {
		return {
			datasourceId: db!.id,
			schemaId,
			tableName: '',
			ddl: '',
			error: `Table or view not found: ${tableName}. Use a name from get_datasource_schemas(...).schemas[].tables[].`
		};
	}

	const meta =
		tableNode.metadata &&
		typeof tableNode.metadata === 'object' &&
		!Array.isArray(tableNode.metadata)
			? (tableNode.metadata as { sql?: string })
			: {};
	const ddl = meta.sql ?? '';

	return {
		datasourceId: db!.id,
		schemaId,
		tableName: tableNode.name ?? '',
		ddl
	};
}

export const getDatasourceTableDetailClient = getDatasourceTableDetailDef.client(
	getDatasourceTableDetailImpl
);
export const getDatasourceTableDetailExecutor = getDatasourceTableDetailImpl;
