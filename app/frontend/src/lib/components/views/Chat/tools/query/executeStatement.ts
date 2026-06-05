import { toolDefinition } from '@tanstack/ai';
import { z } from 'zod';
import { Query } from '$lib/wailsjs/go/db_client/DbClient';
import { tryCatch } from '$lib/utils/tryCatch';
import { toToolError } from '../context';

const statementResultSuccessSchema = z.object({
	success: z.literal(true),
	affectedRows: z
		.number()
		.optional()
		.describe('Rows affected by DML statements. Null for DDL operations like CREATE/ALTER/DROP.'),
	durationMs: z.number().optional(),
	errors: z.array(z.string()).optional().describe('Backend errors or warnings')
});

const executeStatementOutputSchema = z.union([
	z.object({
		error: z.string().describe('Present when statement failed; surface this to the user'),
		success: z.literal(false)
	}),
	statementResultSuccessSchema.describe('Present when statement succeeded')
]);

const queryInputSchema = z.object({
	dbInstanceId: z.string().describe('Database instance ID (required)'),
	statement: z.string().describe('SQL statement to execute (INSERT, UPDATE, DELETE, DDL, etc.)'),
	folderId: z
		.string()
		.nullish()
		.describe(
			'Parent folder ID for variable substitution (optional - only needed if SQL contains variables like $VARIABLE)'
		)
});

export const executeStatementDef = toolDefinition({
	name: 'execute_statement',
	description: `Executes a write or DDL SQL statement (INSERT, UPDATE, DELETE, MERGE, CREATE, ALTER, DROP, TRUNCATE, etc.) against a registered database. Requires user approval before executing.

Before calling:
1. Verify tables with get_database_schemas; use get_database_table_detail(schemaId, tableName) when you need a table's DDL. Never guess schema. Table names may be case-sensitive and require quoting (e.g. "Collaborator").
2. Explain to the user what the statement will do and estimate its impact: how many rows will be affected, what data will change, and whether the operation is reversible.
3. For DROP, TRUNCATE, or DELETE without a WHERE clause, explicitly warn the user that the operation is irreversible and wait for confirmation before proceeding.

Use this for any statement that modifies data or schema. Do not use for read-only SELECT; use execute_query instead.
If error is returned, surface the message to the user and do not proceed.`,
	needsApproval: true,
	inputSchema: queryInputSchema,
	outputSchema: executeStatementOutputSchema
});

type ImplArgs = z.infer<typeof queryInputSchema> & { __fileId?: string };

async function executeStatementImpl(args: unknown) {
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

	return {
		success: true,
		affectedRows: result.affectedRows,
		durationMs: result.durationMs,
		errors: result.errors
	} as z.infer<typeof statementResultSuccessSchema>;
}

export const executeStatementClient = executeStatementDef.client();
export const executeStatementExecutor = executeStatementImpl;
