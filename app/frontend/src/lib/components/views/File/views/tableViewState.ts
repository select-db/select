import type * as graph from '$lib/wails/graph';
import type { Tab } from '$lib/components/Layout/layoutStore';

/**
 * Effective database id for the table view: explicit selection or single DB.
 */
export function getEffectiveSelectedDatasourceId(
	file: graph.FileNode | null | undefined,
	tab: Tab | null | undefined
): string | null {
	if (!file) return null;
	const selected = tab?.file?.activeDatasourceId;
	if (selected != null && selected !== '') return selected;
	const dbs = file.datasources;
	return dbs?.[0]?.id ?? null;
}

/**
 * Query result for the currently selected database.
 * Accepts pre-computed datasourceId to avoid redundant getEffectiveSelectedDatasourceId calls.
 */
export function getQueryResultForDatasource(
	file: graph.FileNode | null | undefined,
	datasourceId: string | null
): graph.QueryResult | null {
	if (!file || !datasourceId) return null;
	return file.queryResults?.[datasourceId] ?? null;
}

/**
 * Plan result for the currently selected database.
 * Accepts pre-computed datasourceId to avoid redundant getEffectiveSelectedDatasourceId calls.
 */
export function getPlanResultForDatasource(
	file: graph.FileNode | null | undefined,
	datasourceId: string | null
): graph.ExplainResult | null {
	if (!file || !datasourceId) return null;
	return file.planResults?.[datasourceId] ?? null;
}

/**
 * Explain result for the currently selected database.
 * Accepts pre-computed datasourceId to avoid redundant getEffectiveSelectedDatasourceId calls.
 */
export function getExplainResultForDatasource(
	file: graph.FileNode | null | undefined,
	datasourceId: string | null
): graph.ExplainResult | null {
	if (!file || !datasourceId) return null;
	return file.explainResults?.[datasourceId] ?? null;
}
