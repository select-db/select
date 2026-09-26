import { runExplain, runPlan, runQuery } from '$lib/utils/query/useQuery';
import { must, tryCatch } from '$lib/utils/tryCatch';
import type * as graph from '$lib/wails/graph';

export const hasDatasource = (file: graph.FileNode | null): boolean => {
	return (file?.datasources?.length ?? 0) > 0;
};

/** All database IDs attached to the file. */
export const getDatasourceIds = (file: graph.FileNode | null): string[] =>
	file?.datasources?.map((d) => d.id).filter((id): id is string => !!id) ?? [];

export type RunStatementParams = {
	statement: string;
	datasourceId: string;
	fileId: string;
	folderId: string;
	explain?: boolean;
	plan?: boolean;
	runtimeVars?: Record<string, string>;
};

export type RunStatementResult =
	| { type: 'query'; result: graph.QueryResult }
	| { type: 'explain'; result: graph.ExplainResult };

export const runStatement = async ({
	statement,
	datasourceId,
	fileId,
	folderId,
	explain,
	plan,
	runtimeVars
}: RunStatementParams): Promise<RunStatementResult | null> => {
	const baseParams = {
		FileID: fileId,
		Statement: statement,
		DatasourceID: datasourceId,
		FolderID: folderId,
		RuntimeVars: runtimeVars ?? {}
	};

	if (explain) {
		const result = await must(tryCatch(runExplain, baseParams));
		return result ? { type: 'explain', result } : null;
	}

	if (plan) {
		const result = await must(tryCatch(runPlan, baseParams));
		return result ? { type: 'explain', result } : null;
	}

	const result = await must(
		tryCatch(runQuery, {
			...baseParams,
			ForExport: false
		})
	);
	return result ? { type: 'query', result } : null;
};
