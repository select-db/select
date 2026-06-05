import { z } from 'zod';

/** CacheStats from dialect/core/explain.go - backend sends *float64/*string, so nullish */
export const cacheStatsSchema = z.looseObject({
	sharedHitBlocks: z.number().optional(),
	sharedReadBlocks: z.number().optional(),
	sharedDirtiedBlocks: z.number().optional(),
	sharedWrittenBlocks: z.number().optional(),
	tempReadBlocks: z.number().optional(),
	tempWriteBlocks: z.number().optional(),
	localHitBlocks: z.number().optional(),
	localReadBlocks: z.number().optional(),
	localDirtiedBlocks: z.number().optional(),
	localWrittenBlocks: z.number().optional(),
	sharedHitBlocksFormatted: z.string().optional(),
	sharedReadBlocksFormatted: z.string().optional(),
	tempReadBlocksFormatted: z.string().optional(),
	tempWriteBlocksFormatted: z.string().optional()
});

export interface ExplainNode {
	id?: string | null;
	type?: string | null;
	operation?: string | null;
	targetTable?: string | null;
	indexName?: string | null;
	condition?: string | null;
	sortKey?: string[] | null;
	estimatedRows?: number | null;
	estimatedCost?: number | null;
	exclusiveCost?: number | null;
	actualRows?: number | null;
	actualTime?: number | null;
	exclusiveTime?: number | null;
	peakMemory?: number | null;
	children?: ExplainNode[] | null;
	metadata?: Record<string, unknown> | null;
	rawOutput?: string | null;
	depth?: number | null;
	percentOfTotal?: number | null;
	warnings?: string[] | null;
	cacheStats?: z.infer<typeof cacheStatsSchema> | null;
}

/** Backend ExplainNode: array fields (sortKey, children, warnings) can be null in JSON. */
export const explainNodeSchema: z.ZodType<ExplainNode> = z.lazy(() =>
	z.looseObject({
		id: z.string().optional(),
		type: z.string().optional(),
		operation: z.string().optional(),
		targetTable: z.string().optional(),
		indexName: z.string().optional(),
		condition: z.string().optional(),
		sortKey: z.array(z.string()).nullish(),
		estimatedRows: z.number().optional(),
		estimatedCost: z.number().optional(),
		exclusiveCost: z.number().optional(),
		actualRows: z.number().optional(),
		actualTime: z.number().optional(),
		exclusiveTime: z.number().optional(),
		peakMemory: z.number().optional(),
		children: z.array(explainNodeSchema).nullish(),
		metadata: z.record(z.string(), z.unknown()).optional(),
		rawOutput: z.string().optional(),
		depth: z.number().optional(),
		percentOfTotal: z.number().optional(),
		warnings: z.array(z.string()).nullish(),
		cacheStats: cacheStatsSchema.optional()
	})
);

/** Shared result shape for explain/plan (id, root, totalCost, errors, durationMs) */
export const explainPlanResultSchema = z.object({
	success: z.literal(true),
	id: z.string().nullish(),
	root: explainNodeSchema.nullish().describe('Parsed plan tree (ExplainNode)'),
	totalCost: z.number().nullish(),
	raw: z.string().nullish(),
	errors: z.array(z.string()).nullish(),
	durationMs: z.number().nullish()
});

export type ExplainPlanResult = z.infer<typeof explainPlanResultSchema>;
