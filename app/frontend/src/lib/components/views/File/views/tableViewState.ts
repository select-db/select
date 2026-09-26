import type * as graph from '$lib/wails/graph';
import type { Tab } from '$lib/components/Layout/layoutStore';

/**
 * Effective database id for the table view: explicit selection or single DB.
 */
export function getEffectiveSelectedDbInstanceId(
	file: graph.FileNode | null | undefined,
	tab: Tab | null | undefined
): string | null {
	if (!file) return null;
	const selected = tab?.file?.activeDbInstanceId;
	if (selected != null && selected !== '') return selected;
	const dbs = file.databases;
	return dbs?.[0]?.id ?? null;
}

/**
 * Query result for the currently selected database.
 * Accepts pre-computed dbInstanceId to avoid redundant getEffectiveSelectedDbInstanceId calls.
 */
export function getQueryResultForDb(
	file: graph.FileNode | null | undefined,
	dbInstanceId: string | null
): graph.QueryResult | null {
	if (!file || !dbInstanceId) return null;
	return file.queryResults?.[dbInstanceId] ?? null;
}

/**
 * Plan result for the currently selected database.
 * Accepts pre-computed dbInstanceId to avoid redundant getEffectiveSelectedDbInstanceId calls.
 */
export function getPlanResultForDb(
	file: graph.FileNode | null | undefined,
	dbInstanceId: string | null
): graph.ExplainResult | null {
	if (!file || !dbInstanceId) return null;
	return file.planResults?.[dbInstanceId] ?? null;
}

/**
 * Explain result for the currently selected database.
 * Accepts pre-computed dbInstanceId to avoid redundant getEffectiveSelectedDbInstanceId calls.
 */
export function getExplainResultForDb(
	file: graph.FileNode | null | undefined,
	dbInstanceId: string | null
): graph.ExplainResult | null {
	if (!file || !dbInstanceId) return null;
	return file.explainResults?.[dbInstanceId] ?? null;
}
