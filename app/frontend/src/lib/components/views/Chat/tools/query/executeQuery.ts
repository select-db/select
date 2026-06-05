import { toolDefinition } from '@tanstack/ai';
import { z } from 'zod';
import { Query } from '$lib/wailsjs/go/db_client/DbClient';
import { tryCatch } from '$lib/utils/tryCatch';
import { toToolError } from '../context';

const MAX_ROWS = 100;

const queryResultSuccessSchema = z.object({
	columns: z.array(z.string()).describe('Column names in order'),
	rows: z
		.array(z.array(z.unknown()))
		.describe('Rows; each row is an array of cell values in column order. May be truncated.'),
	rowCount: z
		.number()
		.optional()
		.describe('Total rows matched. If rowCount > rows.length, results were truncated at 100 rows'),
	durationMs: z.number().optional(),
	errors: z.array(z.string()).optional().describe('Backend errors or warnings')
});

const executeQueryOutputSchema = z.union([
	z.object({
		error: z.string().describe('Present when query failed; surface this to the user'),
		success: z.literal(false)
	}),
	queryResultSuccessSchema.describe('Present when query succeeded')
]);

const queryInputSchema = z.object({
	dbInstanceId: z.string().describe('Database instance ID (required)'),
	statement: z.string().describe('SQL statement to execute (required)'),
	folderId: z
		.string()
		.nullish()
		.describe(
			'Parent folder ID for variable substitution (optional - only needed if SQL contains variables like $VARIABLE)'
		)
});

export const executeQueryDef = toolDefinition({
	name: 'execute_query',
	description: `Executes a read-only SQL SELECT statement against a registered database and returns results capped at 100 rows.

Before calling this tool: verify tables with get_database_schemas; use get_database_table_detail(schemaId, tableName) when you need a table's DDL. Never guess schema. Table names may be case-sensitive and require quoting (e.g. "Collaborator" not collaborator).

Use this to validate queries, explore data, check counts, or debug results. Always run a SELECT here before presenting a query as a final answer to the user.
Never use this for INSERT, UPDATE, DELETE, or DDL; use execute_statement for those.
If rowCount exceeds the returned rows length, results are truncated; inform the user and suggest adding filters or LIMIT.
If error is returned, surface the message to the user and do not proceed.`,
	needsApproval: false,
	inputSchema: queryInputSchema,
	outputSchema: executeQueryOutputSchema
});

type ImplArgs = z.infer<typeof queryInputSchema> & { __fileId?: string };

async function executeQueryImpl(args: unknown) {
	const { dbInstanceId, folderId, statement, __fileId } = args as ImplArgs;
	const fileId = __fileId ?? '';
	const [result, err] = await tryCatch(Query, {
		DbInstanceID: dbInstanceId,
		FileID: fileId,
		FolderID: folderId ?? '',
		Statement: statement,
		ForExport: true, // hack to not cache the result
		RuntimeVars: {}
	});

	if (err) {
		return { error: toToolError(err), success: false };
	}

	const columns = result.columns ?? [];
	const rows = Array.isArray(result.rows) ? result.rows.slice(0, MAX_ROWS) : [];
	return {
		columns,
		rows,
		rowCount: result.rowCount,
		durationMs: result.durationMs,
		errors: result.errors ?? undefined
	} as z.infer<typeof queryResultSuccessSchema>;
}

export const executeQueryExecutor = executeQueryImpl;
