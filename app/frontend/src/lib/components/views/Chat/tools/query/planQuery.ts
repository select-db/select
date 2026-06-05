import { toolDefinition } from '@tanstack/ai';
import { z } from 'zod';
import { Plan } from '$lib/wailsjs/go/db_client/DbClient';
import { tryCatch } from '$lib/utils/tryCatch';
import { toToolError } from '../context';
import { explainPlanResultSchema } from './types';

const queryInputSchema = z.object({
	dbInstanceId: z.string().describe('Database instance ID (required)'),
	statement: z.string().describe('SQL statement to plan (required)'),
	folderId: z
		.string()
		.nullish()
		.describe(
			'Parent folder ID for variable substitution (optional - only needed if SQL contains variables like $VARIABLE)'
		)
});

const planResultSuccessSchema = explainPlanResultSchema;

const planQueryOutputSchema = z.union([
	z.object({
		error: z.string().describe('Error message when the plan failed'),
		success: z.literal(false)
	}),
	planResultSuccessSchema
]);

export const planQueryDef = toolDefinition({
	name: 'plan_query',
	description: `Generates a query execution plan by parsing the query without executing it 
(equivalent to EXPLAIN without ANALYZE). No data is read or modified.

Use this when you want to check query structure, estimated costs, and index usage 
without touching the database; for example on large tables where even a read 
could be expensive, or before the data exists.

For real performance analysis with actual execution stats, use explain_query instead.
It executes the query and returns measured timings and row counts.

Analyze the plan for: sequential scans on large tables, missing indexes, nested 
loop joins on unindexed columns, high estimated row counts. Surface any concerns 
to the user with a concrete suggestion.

Before calling: verify tables with get_database_schemas; use get_database_table_detail(schemaId, tableName) when you need a table's DDL.
If error is returned, surface it to the user and do not proceed.`,
	needsApproval: false,
	inputSchema: queryInputSchema,
	outputSchema: planQueryOutputSchema
});

type ImplArgs = z.infer<typeof queryInputSchema> & { __fileId?: string };

async function planQueryImpl(args: unknown) {
	const { dbInstanceId, folderId, statement, __fileId } = args as ImplArgs;
	const fileId = __fileId ?? '';
	const [r, err] = await tryCatch(Plan, {
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
		success: true as const,
		id: r.id,
		root: r.root,
		totalCost: r.totalCost,
		raw: r.raw,
		errors: r.errors,
		durationMs: r.durationMs
	};
}

export const planQueryExecutor = planQueryImpl;
