import { get } from 'svelte/store';

import { Query, Explain, Plan, CancelQuery, StartQuery } from '$lib/wailsjs/go/db_client/DbClient';
import type { db_client, graph } from '$lib/wailsjs/go/models';

import { notifyError } from '$lib/system/Notifications/notificationsStore';

import { tryCatch } from '../tryCatch';
import { ensureSSHPassphraseForInstance } from '../ssh/passphrase';
import { loadingStore, pushToLoadingStore, removeFromLoadingStore } from './loadingStore';
import { executions, markCancelled, waitForStarted } from './queryStream.svelte';

type RunQueryParams = db_client.QueryParams;
type RunExplainParams = db_client.ExplainParams;
type RunPlanParams = db_client.PlanParams;
type RunOperationParams = RunQueryParams | RunExplainParams | RunPlanParams;

const runOperation = async <T extends RunOperationParams, R>(
	params: T,
	executor: (params: T) => Promise<R>,
	busyMessage: string,
	failureMessage: string,
	retried = false
): Promise<R | null> => {
	const { DbInstanceID, FileID } = params;

	const isLoading = get(loadingStore).some((k) => k.includes(`db:${DbInstanceID}`));
	if (isLoading) {
		notifyError(busyMessage);
		return null;
	}

	pushToLoadingStore(DbInstanceID, FileID);

	const [result, err] = await tryCatch(executor, params);

	removeFromLoadingStore(DbInstanceID, FileID);

	if (err) {
		// Encrypted SSH key: prompt once for the passphrase, then retry.
		if (!retried && (await ensureSSHPassphraseForInstance(DbInstanceID, err.message))) {
			return runOperation(params, executor, busyMessage, failureMessage, true);
		}
		notifyError(failureMessage);
		return null;
	}

	return result;
};

// runQuery starts a streaming query and resolves once the columns are known.
// The returned QueryResult is a "shell": rows come from GetResultPage as the
// streaming cache fills, and totalRowCount becomes final when the backend emits
// 'query:done'. Loading-store cleanup happens in the queryStream event handlers.
//
// ForExport bypasses streaming and returns a fully buffered result.
export const runQuery = async (
	params: RunQueryParams,
	retried = false
): Promise<graph.QueryResult | null> => {
	if (params.ForExport) {
		return runOperation(
			params,
			(p) => Query(p),
			'Query already running on this database',
			'Failed to run query'
		);
	}

	const { DbInstanceID, FileID } = params;

	const isLoading = get(loadingStore).some((k) => k.includes(`db:${DbInstanceID}`));
	if (isLoading) {
		notifyError('Query already running on this database');
		return null;
	}

	pushToLoadingStore(DbInstanceID, FileID);

	const [start, startErr] = await tryCatch(StartQuery, {
		FileID: params.FileID,
		Statement: params.Statement,
		DbInstanceID: params.DbInstanceID,
		FolderID: params.FolderID,
		RuntimeVars: params.RuntimeVars
	});

	if (startErr || !start) {
		removeFromLoadingStore(DbInstanceID, FileID);
		notifyError('Failed to run query');
		return null;
	}

	if (start.errors && start.errors.length > 0) {
		// The backend also emits 'query:error' on prepare failure, but events
		// are async; remove from the loading store here so the UI doesn't
		// briefly show a stuck spinner.
		removeFromLoadingStore(DbInstanceID, FileID);
		// Encrypted SSH key: prompt once for the passphrase, then retry.
		if (!retried && (await ensureSSHPassphraseForInstance(DbInstanceID, start.errors[0]))) {
			return runQuery(params, true);
		}
		notifyError(start.errors[0]);
		return null;
	}

	const [exec, waitErr] = await tryCatch(waitForStarted, start.executionId);
	if (waitErr || !exec) {
		const msg = waitErr?.message ?? 'Failed to run query';
		// Encrypted SSH key: prompt once for the passphrase, then retry.
		if (!retried && (await ensureSSHPassphraseForInstance(DbInstanceID, msg))) {
			removeFromLoadingStore(DbInstanceID, FileID);
			return runQuery(params, true);
		}
		// loading-store removal handled by the 'query:error' event listener
		notifyError(msg);
		return null;
	}

	const queryResult = {
		id: start.executionId,
		columns: exec.columns,
		columnMetadata: exec.columnMetadata,
		rows: [],
		rowCount: 0,
		page: 0,
		pageSize: 75,
		available: 0,
		status: 'streaming'
	} as unknown as graph.QueryResult;

	return queryResult;
};

export const runExplain = async (params: RunExplainParams) =>
	runOperation(
		params,
		(p) => Explain(p),
		'Query already running on this database',
		'Failed to run explain'
	);

export const runPlan = async (params: RunPlanParams) =>
	runOperation(
		params,
		(p) => Plan(p),
		'Query already running on this database',
		'Failed to run plan'
	);

export const cancelQuery = async (params: db_client.CancelQueryParams) => {
	const { DbInstanceID, FileID } = params;

	const [, err] = await tryCatch(CancelQuery, {
		DbInstanceID,
		FileID
	});

	if (err) {
		notifyError('Failed to cancel query');
		return;
	}

	// CancelQuery on the backend triggers ctx cancellation; the resulting
	// engine error flows through 'query:error', which already cleans up the
	// loading store. Mark cancellation locally so the UI can distinguish it
	// from genuine errors before the event arrives.
	const executionId = activeExecutionId(DbInstanceID, FileID);
	if (executionId) markCancelled(executionId);
	else removeFromLoadingStore(DbInstanceID, FileID);

	notifyError('Query cancelled');
};

function activeExecutionId(dbInstanceId: string, fileId: string): string | null {
	for (const id of Object.keys(executions)) {
		const e = executions[id];
		if (e.dbInstanceId === dbInstanceId && e.fileId === fileId && e.status === 'streaming') {
			return id;
		}
	}
	return null;
}
