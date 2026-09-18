import { toolDefinition } from '$lib/components/views/Chat/core/chat/tool-definition';
import { z } from 'zod';
import { loadDatabase } from '../helpers';

const inputSchema = z.object({
	databaseInstanceId: z
		.string()
		.describe('Database instance ID from the context block databases[].id')
});

export const getDatabaseSchemasDef = toolDefinition({
	name: 'get_database_schemas',
	description: `Returns all schemas for a database with table and view names in each. Use this first; then call get_database_table_detail(schemaId, tableName) when you need a table's DDL. If error is returned, surface it to the user and do not proceed.`,
	inputSchema,
	outputSchema: z.object({
		databaseId: z.string(),
		databaseName: z.string(),
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

async function getDatabaseSchemasImpl(args: unknown) {
	const { databaseInstanceId } = args as ImplArgs;

	const { error, db, node } = await loadDatabase(databaseInstanceId);

	if (error) {
		return {
			databaseId: databaseInstanceId,
			databaseName: '',
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
		databaseId: db!.id,
		databaseName: db!.name,
		dialect: db!.dialect,
		schemas: schemaList
	};
}

export const getDatabaseSchemasClient = getDatabaseSchemasDef.client(getDatabaseSchemasImpl);
export const getDatabaseSchemasExecutor = getDatabaseSchemasImpl;
