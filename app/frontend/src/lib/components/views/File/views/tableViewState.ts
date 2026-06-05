import type { graph } from '$lib/wailsjs/go/models';
import type { Tab } from '$lib/components/Layout/layoutStore';

/**
 * Effective database id for the table view: explicit selection or single DB.
 */
export function getEffectiveSelectedDbId(
	file: graph.FileNode | null | undefined,
	tab: Tab | null | undefined
): string | null {
	if (!file) return null;
	const selected = tab?.file?.activeDatabaseId;
	if (selected != null && selected !== '') return selected;
	const dbs = file.databases;
	return dbs?.[0]?.id ?? null;
}

/**
 * Query result for the currently selected database.
 * Accepts pre-computed dbId to avoid redundant getEffectiveSelectedDbId calls.
 */
export function getQueryResultForDb(
	file: graph.FileNode | null | undefined,
	dbId: string | null
): graph.QueryResult | null {
	if (!file || !dbId) return null;
	return file.queryResults?.[dbId] ?? null;
}

/**
 * Plan result for the currently selected database.
 * Accepts pre-computed dbId to avoid redundant getEffectiveSelectedDbId calls.
 */
export function getPlanResultForDb(
	file: graph.FileNode | null | undefined,
	dbId: string | null
): graph.ExplainResult | null {
	if (!file || !dbId) return null;
	return file.planResults?.[dbId] ?? null;
}

/**
 * Explain result for the currently selected database.
 * Accepts pre-computed dbId to avoid redundant getEffectiveSelectedDbId calls.
 */
export function getExplainResultForDb(
	file: graph.FileNode | null | undefined,
	dbId: string | null
): graph.ExplainResult | null {
	if (!file || !dbId) return null;
	return file.explainResults?.[dbId] ?? null;
}
