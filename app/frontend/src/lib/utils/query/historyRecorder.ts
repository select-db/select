import { get } from 'svelte/store';

import { CreateHistory } from '$lib/bindings/selectDb/internal/history/history';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { bumpHistoryRefresh } from '$lib/components/views/History/historyStore';

// Statements aren't carried on the backend query events, so we stash them here
// when a query starts and pair them with the terminal done/error outcome to
// record local history. Keyed by executionId.
interface PendingStatement {
	statement: string;
	dbInstanceId: string;
}

const pending = new Map<string, PendingStatement>();

export function registerPendingHistory(
	executionId: string,
	statement: string,
	dbInstanceId: string
): void {
	pending.set(executionId, { statement, dbInstanceId });
}

export interface HistoryOutcome {
	affectedRows?: number;
	rowCount?: number;
	durationMs?: number;
	errors: string[];
}

/**
 * Records a finished statement against local history. Status is derived from
 * the presence of errors. Best-effort: failures are swallowed so history never
 * disrupts query execution. The database name/type are resolved on read from
 * the workspace graph, so only DbInstanceID is persisted here.
 */
export function recordHistory(executionId: string, outcome: HistoryOutcome): void {
	const entry = pending.get(executionId);
	if (!entry) return;
	pending.delete(executionId);

	const workspaceId = get(workspaceGraphStore)?.id ?? '';

	CreateHistory({
		Dsn: '',
		Uri: '',
		WorkspaceID: workspaceId,
		DbInstanceID: entry.dbInstanceId,
		Statement: entry.statement,
		AffectedRows: outcome.affectedRows ?? null,
		RowCount: outcome.rowCount ?? null,
		DurationMs: outcome.durationMs ?? null,
		Errors: outcome.errors
	})
		.then(() => bumpHistoryRefresh())
		.catch(() => {});
}
