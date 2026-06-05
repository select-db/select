import { runExplain, runPlan, runQuery } from '$lib/utils/query/useQuery';
import { must, tryCatch } from '$lib/utils/tryCatch';
import type { graph } from '$lib/wailsjs/go/models';

export const hasDatabase = (file: graph.FileNode | null): boolean => {
	return (file?.databases?.length ?? 0) > 0;
};

/** All database IDs attached to the file. */
export const getDbIds = (file: graph.FileNode | null): string[] =>
	file?.databases?.map((d) => d.id).filter((id): id is string => !!id) ?? [];

export type RunStatementParams = {
	statement: string;
	dbInstanceId: string;
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
	dbInstanceId,
	fileId,
	folderId,
	explain,
	plan,
	runtimeVars
}: RunStatementParams): Promise<RunStatementResult | null> => {
	const baseParams = {
		FileID: fileId,
		Statement: statement,
		DbInstanceID: dbInstanceId,
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
