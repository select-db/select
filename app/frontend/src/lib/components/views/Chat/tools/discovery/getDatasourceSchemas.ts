import { toolDefinition } from '$lib/components/views/Chat/core/chat/tool-definition';
import { z } from 'zod';
import { loadDatasource } from '../helpers';

const inputSchema = z.object({
	datasourceId: z.string().describe('Datasource ID from the context block datasources[].id')
});

export const getDatasourceSchemasDef = toolDefinition({
	name: 'get_datasource_schemas',
	description: `Returns all schemas for a database with table and view names in each. Use this first; then call get_datasource_table_detail(schemaId, tableName) when you need a table's DDL. If error is returned, surface it to the user and do not proceed.`,
	inputSchema,
	outputSchema: z.object({
		datasourceId: z.string(),
		datasourceName: z.string(),
		dialect: z
			.string()
			.describe('SQL dialect e.g. postgres, mysql, sqlite. Use when writing queries'),
		schemas: z.array(
			z.object({
				id: z.string(),
				name: z.string(),
				tables: z.array(z.string()).describe('Table and view names in this schema')
			})
		),
		error: z.string().optional()
	})
});

type ImplArgs = z.infer<typeof inputSchema>;

async function getDatasourceSchemasImpl(args: unknown) {
	const { datasourceId } = args as ImplArgs;

	const { error, db, node } = await loadDatasource(datasourceId);

	if (error) {
		return {
			datasourceId: datasourceId,
			datasourceName: '',
			dialect: '',
			schemas: [],
			error
		};
	}

	const schemaList = node!.children
		.filter((c) => c.type === 'schema')
		.map((schemaNode) => {
			const tablesGroup = schemaNode.children.find((c) => c.type === 'tables');
			const viewsGroup = schemaNode.children.find((c) => c.type === 'views');
			const tableNames = (tablesGroup?.children ?? []).map((c) => c.name ?? '').filter(Boolean);
			const viewNames = (viewsGroup?.children ?? []).map((c) => c.name ?? '').filter(Boolean);
			const tables = [...tableNames, ...viewNames];
			return {
				id: schemaNode.id,
				name: schemaNode.name,
				tables
			};
		});

	return {
		datasourceId: db!.id,
		datasourceName: db!.name,
		dialect: db!.dialect,
		schemas: schemaList
	};
}

export const getDatasourceSchemasClient = getDatasourceSchemasDef.client(getDatasourceSchemasImpl);
export const getDatasourceSchemasExecutor = getDatasourceSchemasImpl;
