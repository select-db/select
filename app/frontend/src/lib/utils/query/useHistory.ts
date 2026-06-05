import { CreateHistory } from '$lib/wailsjs/go/history/History';
import type { history } from '$lib/wailsjs/go/models';
import type { graph } from '$lib/wailsjs/go/models';

type CreateHistoryArgs = {
	dsn: string;
	uri: string;
	statement: string;
	result: graph.QueryResult;
};

export const createHistory = async ({ dsn, uri, statement, result }: CreateHistoryArgs) => {
	const params: history.CreateQueryHistoryParams = {
		Dsn: dsn,
		Uri: uri,
		Statement: statement,
		AffectedRows: result.affectedRows,
		RowCount: result.rowCount,
		DurationMs: result.durationMs,
		Errors: result.errors ?? []
	};
	await CreateHistory(params);
};
