<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { addChatTab } from '$lib/components/Layout/layoutStore';
	import type { graph } from '$lib/wailsjs/go/models';

	import { loadingStore, toKey } from '$lib/utils/query/loadingStore';
	import { getExecution } from '$lib/utils/query/queryStream.svelte';

	import ExplainTable from './ExplainTable.svelte';
	import ResultsTable from './ResultsTable.svelte';
	import TableErrorRow from './TableErrorRow.svelte';
	import TableEmptyState from './TableEmptyState.svelte';
	import GraphView from './Graph/GraphView.svelte';

	type TableProps = {
		tableHeight?: number;
		tab: Tab;
		effectiveDbId: string | null;
		currentQueryResult: graph.QueryResult | null;
		currentPlanResult: graph.ExplainResult | null;
		currentExplainResult: graph.ExplainResult | null;
		run: (mode: 'run' | 'explain' | 'plan') => void;
		content?: string;
	};
	let {
		tableHeight = 0,
		tab,
		effectiveDbId,
		currentQueryResult,
		currentPlanResult,
		currentExplainResult,
		run,
		content = ''
	}: TableProps = $props();

	const file = $derived(tab.file?.node);
	const activeView = $derived(tab.file?.viewMode ?? 'results');

	const loading = $derived($loadingStore.includes(toKey(effectiveDbId ?? undefined, file?.id)));
	const queryResult = $derived(currentQueryResult);
	const planResult = $derived(currentPlanResult);
	const explainResult = $derived(currentExplainResult);

	function openFixInChat(error: string) {
		addChatTab({
			databaseId: effectiveDbId,
			action:
				activeView === 'results' ? 'fix-query' : activeView === 'plan' ? 'fix-plan' : 'fix-explain',
			error,
			instruction:
				activeView === 'results'
					? 'Fix the query so it runs without error.'
					: activeView === 'plan'
						? 'Fix the plan so it runs without error.'
						: 'Fix the explain so it runs without error.'
		});
	}
</script>

<div class="table-container" style="height: {tableHeight}px;">
	{#if activeView === 'results'}
		{@const execution = getExecution(queryResult?.id)}
		{@const liveRowCount =
			execution?.totalRowCount ?? execution?.available ?? queryResult?.rowCount ?? 0}
		{@const hasColumns = (queryResult?.columns?.length ?? 0) > 0}
		{@const hasRows = hasColumns && liveRowCount > 0}
		{@const isStreaming = execution?.status === 'streaming'}
		{@const affectedRows = execution?.affectedRows ?? queryResult?.affectedRows}
		{@const errorMessage = execution?.error ?? queryResult?.errors?.[0]}
		{@const affectedOnly =
			!hasRows &&
			!isStreaming &&
			affectedRows !== undefined &&
			(affectedRows > 0 || !hasColumns)}
		{#if loading && !execution}
			<p class="placeholder">Loading...</p>
		{:else if queryResult}
			{#if errorMessage}
				<TableErrorRow
					error={errorMessage}
					onFixInChat={openFixInChat}
					onRun={() => run('run')}
				/>
			{/if}
			{#if affectedOnly}
				<p class="placeholder">{affectedRows} row(s) affected.</p>
			{:else if hasColumns}
				<!-- Also the zero-row case: a header row with nothing under it reads as
				     "ran, matched nothing", where a text placeholder read like the
				     not-run-yet state. -->
				<ResultsTable {tab} />
			{:else if !errorMessage && !isStreaming}
				<p class="placeholder muted">No rows returned.</p>
			{/if}
		{:else}
			<TableEmptyState
				message="No result yet."
				button={{
					onclick: () => run('run')
				}}
			/>
		{/if}
	{:else if activeView === 'plan'}
		{#if planResult?.errors}
			<TableErrorRow
				error={planResult.errors[0]}
				onFixInChat={openFixInChat}
				onRun={() => run('plan')}
			/>
		{:else if planResult?.root}
			<ExplainTable result={planResult} />
		{:else}
			<TableEmptyState
				message="No plan result yet."
				button={{
					onclick: () => run('plan')
				}}
			/>
		{/if}
	{:else if activeView === 'explain'}
		{#if explainResult?.errors}
			<TableErrorRow
				error={explainResult.errors[0]}
				onFixInChat={openFixInChat}
				onRun={() => run('explain')}
			/>
		{:else if explainResult?.root}
			<ExplainTable result={explainResult} />
		{:else}
			<TableEmptyState
				message="No explain analyse result yet."
				button={{
					onclick: () => run('explain')
				}}
			/>
		{/if}
	{:else if activeView === 'graph'}
		<GraphView {tab} {effectiveDbId} queryResult={currentQueryResult} {content} onRun={() => run('run')} />
	{/if}
</div>

<style>
	.table-container {
		overflow: hidden;
		position: relative;
	}

	.placeholder {
		padding: var(--space-md);
		font-weight: var(--fw-light);
	}

	.placeholder.muted {
		color: var(--gray-800);
	}
</style>
