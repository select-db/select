import { clientTools } from '@tanstack/ai-client';

import {
	getDatabaseSchemasClient,
	getDatabaseSchemasExecutor
} from './discovery/getDatabaseSchemas';
import {
	getDatabaseTableDetailClient,
	getDatabaseTableDetailExecutor
} from './discovery/getDatabaseTableDetail';
import { readFileClient, readFileExecutor } from './discovery/readFile';
import type { OnApprovalRequested } from '$lib/components/views/Chat/composables/useToolExecution.svelte';
import {
	editFileClient,
	editFileExecutor,
	createEditFileApprovalHandler
} from './discovery/editFile';
import { executeCommandClient, executeCommandExecutor } from './executeCommand';
import type { QueryRegistry } from './query/queryRegistry';
import { executeQueryDef, executeQueryExecutor } from './query/executeQuery';
import { executeStatementClient, executeStatementExecutor } from './query/executeStatement';
import { explainQueryClient, explainQueryExecutor } from './query/explainQuery';
import { planQueryDef, planQueryExecutor } from './query/planQuery';

export type { ChatContext } from './context';
export { createDefaultGetContext, getContextForChatTab } from './context';

type QueryExecutorArgs = {
	dbInstanceId: string;
	statement: string;
	folderId?: string | null;
	__fileId?: string;
};

function wrapQueryExecutor(
	executor: (args: QueryExecutorArgs) => Promise<unknown>,
	sessionId: string,
	registry: QueryRegistry
): (args: unknown) => Promise<unknown> {
	return async (args: unknown) => {
		const a = args as QueryExecutorArgs;
		const fileId = `chat://${sessionId}/${crypto.randomUUID()}`;
		registry.add(a.dbInstanceId, fileId);
		try {
			return await executor({ ...a, __fileId: fileId });
		} finally {
			registry.remove(a.dbInstanceId, fileId);
		}
	};
}

/** Factory: create tools and executors. Requires sessionId and queryRegistry for cancellation. */
export function createChatTools(sessionId: string, queryRegistry: QueryRegistry) {
	const wrappedExecuteQuery = wrapQueryExecutor(executeQueryExecutor, sessionId, queryRegistry);
	const wrappedExecuteStatement = wrapQueryExecutor(
		executeStatementExecutor,
		sessionId,
		queryRegistry
	);
	const wrappedExplainQuery = wrapQueryExecutor(explainQueryExecutor, sessionId, queryRegistry);
	const wrappedPlanQuery = wrapQueryExecutor(planQueryExecutor, sessionId, queryRegistry);

	const tools = clientTools(
		getDatabaseSchemasClient,
		getDatabaseTableDetailClient,
		readFileClient,
		editFileClient,
		executeCommandClient,
		executeQueryDef.client(wrappedExecuteQuery),
		executeStatementClient,
		explainQueryClient,
		planQueryDef.client(wrappedPlanQuery)
	);

	const toolExecutors: Record<string, (args: unknown, ctx?: unknown) => Promise<unknown>> = {
		get_database_schemas: getDatabaseSchemasExecutor,
		get_database_table_detail: getDatabaseTableDetailExecutor,
		read_file: readFileExecutor,
		edit_file: editFileExecutor as (args: unknown, ctx?: unknown) => Promise<unknown>,
		execute_command: executeCommandExecutor,
		execute_query: wrappedExecuteQuery,
		execute_statement: wrappedExecuteStatement,
		explain_query: wrappedExplainQuery,
		plan_query: wrappedPlanQuery
	};

	/** Per-tool hook called when a tool enters approval-requested state (e.g. open diff UI). */
	const onApprovalRequestedHandlers: Record<string, OnApprovalRequested> = {
		edit_file: createEditFileApprovalHandler(sessionId)
	};

	return { tools, toolExecutors, onApprovalRequestedHandlers };
}
