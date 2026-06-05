import { EventsOn } from '$lib/wailsjs/runtime/runtime';
import type { graph } from '$lib/wailsjs/go/models';
import { removeFromLoadingStore } from './loadingStore';

export type QueryExecutionStatus = 'streaming' | 'done' | 'error' | 'cancelled';

export interface QueryExecution {
	id: string;
	dbInstanceId: string;
	fileId: string;
	columns: string[];
	columnMetadata: graph.ColumnMetadata[];
	available: number;
	totalRowCount: number | null;
	status: QueryExecutionStatus;
	error?: string;
	errorPosition?: number;
	affectedRows?: number;
	durationMs?: number;
}

interface StartedPayload {
	executionId: string;
	dbInstanceId: string;
	fileId: string;
	columns: string[];
	columnMetadata?: graph.ColumnMetadata[];
}
interface ExecutedPayload {
	executionId: string;
	dbInstanceId: string;
	fileId: string;
	durationMs: number;
}
interface ProgressPayload {
	executionId: string;
	dbInstanceId: string;
	fileId: string;
	available: number;
}
interface DonePayload {
	executionId: string;
	dbInstanceId: string;
	fileId: string;
	rowCount: number;
	affectedRows: number;
	durationMs: number;
}
interface ErrorPayload {
	executionId: string;
	dbInstanceId: string;
	fileId: string;
	message: string;
	errorPosition?: number;
}

// Module-scoped reactive registry. Keyed by executionId. The table looks up
// the current execution via getExecution() and reacts to its mutations.
export const executions = $state<Record<string, QueryExecution>>({});

interface StartedWaiter {
	resolve: (e: QueryExecution) => void;
	reject: (err: Error) => void;
}
const startedWaiters = new Map<string, StartedWaiter>();

EventsOn('query:started', (e: StartedPayload) => {
	const previous = executions[e.executionId];
	const exec: QueryExecution = {
		id: e.executionId,
		dbInstanceId: e.dbInstanceId,
		fileId: e.fileId,
		columns: e.columns ?? [],
		columnMetadata: e.columnMetadata ?? [],
		available: previous?.available ?? 0,
		totalRowCount: previous?.totalRowCount ?? null,
		status: 'streaming',
		// Preserve a duration that may have arrived ahead of started (defensive
		// against any backend reordering of the executed/started events).
		durationMs: previous?.durationMs,
		affectedRows: previous?.affectedRows
	};
	executions[e.executionId] = exec;

	const waiter = startedWaiters.get(e.executionId);
	if (waiter) {
		startedWaiters.delete(e.executionId);
		waiter.resolve(exec);
	}
});

EventsOn('query:executed', (e: ExecutedPayload) => {
	let exec = executions[e.executionId];
	if (!exec) {
		// Buffer the duration via a placeholder; query:started will preserve it.
		exec = {
			id: e.executionId,
			dbInstanceId: e.dbInstanceId,
			fileId: e.fileId,
			columns: [],
			columnMetadata: [],
			available: 0,
			totalRowCount: null,
			status: 'streaming',
			durationMs: e.durationMs
		};
		executions[e.executionId] = exec;
		return;
	}
	// First-write wins so the final summary doesn't overwrite the early
	// SQL-execution duration with the (longer) total streaming time.
	if (exec.durationMs == null || exec.durationMs === 0) {
		exec.durationMs = e.durationMs;
	}
});

EventsOn('query:progress', (e: ProgressPayload) => {
	const exec = executions[e.executionId];
	if (!exec) return;
	if (e.available > exec.available) exec.available = e.available;
});

EventsOn('query:done', (e: DonePayload) => {
	const exec = executions[e.executionId];
	if (!exec) {
		removeFromLoadingStore(e.dbInstanceId, e.fileId);
		return;
	}
	exec.available = Math.max(exec.available, e.rowCount);
	exec.totalRowCount = e.rowCount;
	exec.affectedRows = e.affectedRows;
	// Prefer the early SQL duration if we already received one; only fall
	// back to the summary value when the early hook never fired (proxified).
	if (exec.durationMs == null || exec.durationMs === 0) {
		exec.durationMs = e.durationMs;
	}
	exec.status = 'done';
	removeFromLoadingStore(exec.dbInstanceId, exec.fileId);
});

EventsOn('query:error', (e: ErrorPayload) => {
	const existing = executions[e.executionId];
	if (existing) {
		existing.status = 'error';
		existing.error = e.message;
		existing.errorPosition = e.errorPosition;
	} else {
		executions[e.executionId] = {
			id: e.executionId,
			dbInstanceId: e.dbInstanceId,
			fileId: e.fileId,
			columns: [],
			columnMetadata: [],
			available: 0,
			totalRowCount: 0,
			status: 'error',
			error: e.message,
			errorPosition: e.errorPosition
		};
	}

	const waiter = startedWaiters.get(e.executionId);
	if (waiter) {
		startedWaiters.delete(e.executionId);
		waiter.reject(new Error(e.message));
	}

	removeFromLoadingStore(e.dbInstanceId, e.fileId);
});

// waitForStarted resolves with the execution once 'query:started' fires.
// Rejects if 'query:error' fires before 'query:started'.
export function waitForStarted(executionId: string): Promise<QueryExecution> {
	const existing = executions[executionId];
	if (existing) {
		if (existing.status === 'error') return Promise.reject(new Error(existing.error ?? 'query failed'));
		return Promise.resolve(existing);
	}
	return new Promise((resolve, reject) => {
		startedWaiters.set(executionId, { resolve, reject });
	});
}

export function getExecution(id: string | null | undefined): QueryExecution | null {
	if (!id) return null;
	return executions[id] ?? null;
}

export function markCancelled(id: string) {
	const exec = executions[id];
	if (!exec) return;
	exec.status = 'cancelled';
	removeFromLoadingStore(exec.dbInstanceId, exec.fileId);
}

// dropExecution removes an execution from the registry.
// Currently unused but exposed for cache eviction handling later.
export function dropExecution(id: string) {
	delete executions[id];
}
