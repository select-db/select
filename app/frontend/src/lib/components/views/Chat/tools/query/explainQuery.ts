import { toolDefinition } from '$lib/components/views/Chat/core/chat/tool-definition';
import { z } from 'zod';
import { Explain } from '$lib/wailsjs/go/db_client/DbClient';
import { tryCatch } from '$lib/utils/tryCatch';
import { toToolError } from '../context';
import { explainPlanResultSchema } from './types';

const queryInputSchema = z.object({
	dbInstanceId: z.string().describe('Database instance ID (required)'),
	statement: z.string().describe('SQL statement to explain (required)'),
	folderId: z
		.string()
		.nullish()
		.describe(
			'Parent folder ID for variable substitution (optional - only needed if SQL contains variables like $VARIABLE)'
		)
});

const explainResultSuccessSchema = explainPlanResultSchema;

const explainQueryOutputSchema = z.union([
	z.object({
		error: z.string().describe('Error message when the explain failed'),
		success: z.literal(false)
	}),
	explainResultSuccessSchema
]);

export const explainQueryDef = toolDefinition({
	name: 'explain_query',
	description:
		'Explain a SQL query (execute and show execution plan). Only dbInstanceId and statement are required. folderId is optional for variable substitution. Verify tables with get_database_schemas and get_database_table_detail before running queries. Table names may be case-sensitive and require quotes.',
	needsApproval: true,
	inputSchema: queryInputSchema,
	outputSchema: explainQueryOutputSchema
});

type ImplArgs = z.infer<typeof queryInputSchema> & { __fileId?: string };

async function explainQueryImpl(args: unknown) {
	const { dbInstanceId, folderId, statement, __fileId } = args as ImplArgs;
	const fileId = __fileId ?? '';
	const [r, err] = await tryCatch(Explain, {
		DbInstanceID: dbInstanceId,
		FileID: fileId,
		FolderID: folderId ?? '',
		Statement: statement,
		RuntimeVars: {}
	});

	if (err) {
		return { error: toToolError(err), success: false };
	}

	return {
		success: true,
		id: r.id,
		root: r.root,
		totalCost: r.totalCost,
		raw: r.raw,
		errors: r.errors,
		durationMs: r.durationMs
	};
}

export const explainQueryClient = explainQueryDef.client();
export const explainQueryExecutor = explainQueryImpl;
