<script lang="ts">
	import type { Component } from 'svelte';
	import * as dbc from '$lib/wailsjs/go/db_client/DbClient';
	import { db_client } from '$lib/wailsjs/go/models';
	import * as graphApi from '$lib/wailsjs/go/graph/Graph';
	import Button from '$lib/system/Button/Button.svelte';
	import SegmentedControl from '$lib/system/SegmentedControl/SegmentedControl.svelte';

	import {
		addChatTab,
		addTempFileTab,
		updateTab,
		type Tab
	} from '$lib/components/Layout/layoutStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import RuntimeVarsModal from '../../RuntimeVarsModal.svelte';

	import { loadingStore, toKey } from '$lib/utils/query/loadingStore';
	import { getExecution } from '$lib/utils/query/queryStream.svelte';

	import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
	import Contextable from '$lib/system/ContextMenu/Contextable.svelte';
	import TableTimer from './TableTimer.svelte';
	import Resizer from './Resizer.svelte';
	import DatabaseBadge from './DatabaseBadge.svelte';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import {
		getEffectiveSelectedDbId,
		getExplainResultForDb,
		getPlanResultForDb,
		getQueryResultForDb
	} from '../../views/tableViewState';

	type TableHeaderProps = {
		tab: Tab;
		run: (type: 'run' | 'explain' | 'plan', dbIds?: string[]) => Promise<void>;
		tableHeight?: number;
		content?: string;
	};
	let { tab, run, tableHeight = $bindable(0), content = '' }: TableHeaderProps = $props();

	const file = $derived(tab.file?.node);
	const exportFilename = $derived(file?.name?.replace(/\.sql$/i, '') ?? 'query_results');
	const effectiveDbId = $derived(getEffectiveSelectedDbId(file, tab));
	const queryResult = $derived(getQueryResultForDb(file, effectiveDbId));
	const planResult = $derived(getPlanResultForDb(file, effectiveDbId));
	const explainResult = $derived(getExplainResultForDb(file, effectiveDbId));
	const activeView = $derived(tab.file?.viewMode ?? 'results');

	const tableState = $derived(effectiveDbId ? (tab.file?.tables?.[effectiveDbId] ?? null) : null);

	const edits = $derived(tableState?.edits ?? {});
	const editedRowCount = $derived.by(() => {
		const rows = new Set<number>();
		for (const key of Object.keys(edits)) {
			const rowIndex = parseInt(key.split(':')[0], 10);
			rows.add(rowIndex);
		}
		return rows.size;
	});

	const loading = $derived($loadingStore.includes(toKey(effectiveDbId ?? undefined, file?.id)));

	const databaseIds = $derived(
		[...(file?.databases ?? [])]
			.sort((a, b) => (a.name ?? '').localeCompare(b.name ?? ''))
			.map((d) => d.id)
			.filter((id): id is string => !!id)
	);

	const hasDbError = (dbId: string): boolean => {
		const result =
			activeView === 'plan'
				? getPlanResultForDb(file, dbId)
				: activeView === 'explain'
					? getExplainResultForDb(file, dbId)
					: getQueryResultForDb(file, dbId);
		return (result?.errors?.length ?? 0) > 0;
	};

	const setActiveDatabase = (e: MouseEvent, dbId: string) => {
		e.preventDefault();
		e.stopPropagation();

		if (!tab.file) return;

		updateTab({
			...tab,
			file: {
				...tab.file,
				activeDatabaseId: dbId
			}
		});
	};

	// Live execution snapshot. While streaming, durationMs is set as soon as
	// 'query:executed' fires (right when the SQL finished, before all rows
	// have streamed); rowCount tracks the watermark and becomes the true total
	// at 'query:done'.
	const execution = $derived(getExecution(queryResult?.id));
	const rowCount = $derived(
		execution?.totalRowCount ?? execution?.available ?? queryResult?.rowCount ?? 0
	);
	const liveDurationMs = $derived(execution?.durationMs ?? queryResult?.durationMs);

	const setViewMode = (viewMode: 'results' | 'plan' | 'explain' | 'graph') => {
		if (!tab.file) return;

		updateTab({
			...tab,
			file: {
				...tab.file,
				viewMode
			}
		});
		if (tableHeight !== 0) return;

		tableHeight = 175;
	};

	const toggleTable = () => {
		tableHeight = tableHeight > 0 ? 0 : 175;
	};

	const rollbackEdits = () => {
		if (!tab.file) return;
		if (!effectiveDbId) return;

		updateTab({
			...tab,
			file: {
				...tab.file,
				tables: {
					...(tab.file.tables ?? {}),
					[effectiveDbId]: {
						...(tab.file.tables?.[effectiveDbId] ?? {}),
						edits: {}
					}
				}
			}
		});
	};

	const reviewEdits = async () => {
		if (!effectiveDbId || !file) return;

		const params = db_client.GenerateUpdateSQLParams.createFrom({
			databaseId: effectiveDbId,
			edits: Object.values(edits)
		});

		const { sql } = await must(tryCatch(dbc.GenerateUpdateSQL, params));

		addTempFileTab(
			{
				content: sql,
				name: `[edits].sql`,
				dbInstanceId: effectiveDbId,
				folderId: file.folder_id ?? ''
			},
			false
		);
	};

	const analyzeWithChat = () => {
		if (!effectiveDbId) return;

		const hasErrors =
			activeView === 'plan'
				? (planResult?.errors?.length ?? 0) > 0
				: (explainResult?.errors?.length ?? 0) > 0;

		addChatTab({
			databaseId: effectiveDbId,
			action: activeView === 'plan' ? 'analyze-plan' : 'analyze-explain',
			hasErrors,
			instruction:
				activeView === 'plan'
					? 'Analyze this query plan and suggest performance and index improvements. Be super consise. '
					: 'Analyze this EXPLAIN ANALYZE result and suggest performance and index improvements. Be super consise. '
		});
	};

	const exportFormatConfig = [
		{ label: 'CSV (;)', format: 'csv_semicolon', ext: '.csv' },
		{ label: 'CSV (,)', format: 'csv_comma', ext: '.csv' },
		{ label: 'JSON', format: 'json', ext: '.json' }
	];

	const varPattern = /\$([A-Za-z_][A-Za-z0-9_]*)/g;

	const promptRuntimeVars = (
		vars: string[],
		initial: Record<string, string>,
		initialTypes: Record<string, string>
	): Promise<{ vals: Record<string, string>; types: Record<string, string> } | null> =>
		new Promise((resolve) => {
			modalStore.set({
				content: (() => RuntimeVarsModal) as () => Component,
				props: {
					vars,
					initial,
					initialTypes,
					onSubmit: (vals: Record<string, string>, types: Record<string, string>) => {
						modalStore.set(null);
						resolve({ vals, types });
					},
					onCancel: () => {
						modalStore.set(null);
						resolve(null);
					}
				},
				width: 440
			});
		});

	const exportOptions = $derived<ContextMenuOption[]>(
		exportFormatConfig.map(({ label, format, ext }) => ({
			label,
			action: async (onClose: () => void) => {
				onClose();

				const folderId = file?.folder_id ?? '';
				const dbInstanceId = effectiveDbId ?? '';

				const [resolvedVars] = await tryCatch(graphApi.GetUriVariables, file?.uri ?? '');
				const resolvedNames = new Set((resolvedVars ?? []).map((v) => v.name));
				const allVarNames = [...new Set([...content.matchAll(varPattern)].map((m) => m[1]))];
				const unresolvedNames = allVarNames.filter((name) => !resolvedNames.has(name));

				let runtimeVars: Record<string, string> = { ...(tab.file?.runtimeVars ?? {}) };
				if (unresolvedNames.length > 0) {
					// Always prompt, stored values pre-fill the form
					const result = await promptRuntimeVars(
						unresolvedNames,
						runtimeVars,
						tab.file?.runtimeVarTypes ?? {}
					);
					if (result === null) return;
					runtimeVars = { ...runtimeVars, ...result.vals };
					if (tab.file) {
						updateTab({
							...tab,
							file: {
								...tab.file,
								runtimeVars,
								runtimeVarTypes: { ...(tab.file.runtimeVarTypes ?? {}), ...result.types }
							}
						});
					}
				}

				await must(
					tryCatch(dbc.Export, {
						FileID: file?.id ?? '',
						Statement: content,
						DbInstanceID: dbInstanceId,
						FolderID: folderId,
						Format: format,
						Filename: exportFilename + ext,
						RuntimeVars: runtimeVars
					})
				);
			}
		}))
	);
</script>

<div class="headerWrapper">
	<Resizer bind:height={tableHeight} {tab} />

	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="tableHeader" onclick={toggleTable}>
		{#if databaseIds.length > 1}
			<div class="db-badges">
				{#each databaseIds as dbId (dbId)}
					<DatabaseBadge
						{dbId}
						{run}
						active={dbId === effectiveDbId}
						error={hasDbError(dbId)}
						onclick={(e) => setActiveDatabase(e, dbId)}
					/>
				{/each}
			</div>
		{/if}
		<div class="wrapper">
			<TableTimer
				{loading}
				durationMs={activeView === 'plan'
					? planResult?.durationMs
					: activeView === 'explain'
						? explainResult?.durationMs
						: liveDurationMs}
			/>
			<SegmentedControl
				options={[
					{
						id: 'results',
						icon: 'column-1',
						label: loading && rowCount === 0 ? `...` : rowCount.toLocaleString(),
						tooltip: 'Run result'
					},
					{
						id: 'graph',
						icon: 'chart-line',
						tooltip: 'Graph view'
					},

					{
						id: 'plan',
						icon: 'map',
						tooltip: 'Plan result'
					},
					{
						id: 'explain',
						icon: 'chart',
						tooltip: 'Analyze result'
					}
				]}
				value={activeView}
				onSelect={(value) => setViewMode(value as 'results' | 'plan' | 'explain' | 'graph')}
			/>
			<div class="right-wrapper">
				{#if activeView === 'results'}
					{#if editedRowCount > 0}
						<Button
							size="sm"
							content="Review"
							leftIcon="db-upload"
							iconSize={16}
							onclick={reviewEdits}
						/>
						<p>or</p>
						<Button
							size="sm"
							content="Rollback"
							leftIcon="refresh"
							iconSize={16}
							onclick={rollbackEdits}
						/>
						<p>{`${editedRowCount} edit${editedRowCount !== 1 ? 's' : ''}`}</p>
					{/if}
					{#if queryResult && rowCount > 0}
						<Contextable options={exportOptions} direction="left" on="click" menuWidth={100}>
							<Button leftIcon="download" iconSize={17} label="Export" />
						</Contextable>
					{/if}
				{:else if activeView === 'plan' && planResult && !planResult.errors?.length}
					<Button
						content="Analyze in chat"
						leftIcon="chat"
						iconSize={16}
						onclick={analyzeWithChat}
					/>
				{:else if activeView === 'explain' && explainResult && !explainResult.errors?.length}
					<Button
						content="Analyze in chat"
						leftIcon="chat"
						iconSize={16}
						onclick={analyzeWithChat}
					/>
				{/if}
			</div>
		</div>
	</div>
</div>

{#if queryResult || planResult || explainResult}
	{@const hasErrors =
		activeView === 'plan'
			? planResult?.errors
			: activeView === 'explain'
				? explainResult?.errors
				: queryResult?.errors}
	<div class={`statusBar ${loading ? 'loading' : hasErrors ? 'error' : 'success'}`}></div>
{:else}
	<div class="divider" class:border-top={tableHeight !== 0}></div>
{/if}

<style>
	.db-badges {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: var(--space-xs);
		padding: var(--space-sm) 0 var(--space-xxs) 0;
		margin-bottom: -2px;
	}

	.tableHeader {
		padding: 0 var(--space-xs-sm) 0 var(--space-sm-md);
		border-top: var(--border);
		background-color: var(--gray-0);

		transition: background-color 0.1s ease-in;
	}

	:global(.tableHeader button) {
		height: 26px;
	}

	.tableHeader:hover {
		background-color: var(--gray-100);
	}

	.wrapper {
		height: 43px;

		display: flex;
		gap: var(--space-xs);
		align-items: center;
	}

	.statusBar {
		width: 100%;
		height: 0;
		border-top: 0.5px var(--white-glow) solid;
	}
	.statusBar.error {
		border-color: var(--red-glow);
	}

	.statusBar.success {
		border-color: var(--green-glow);
	}

	.divider {
		width: 100%;
		height: 0.5px;
		border-top: none;
	}
	.divider.border-top {
		height: 0;
		border-top: var(--border);
	}

	.headerWrapper {
		position: relative;
	}

	.right-wrapper {
		display: flex;
		align-items: center;
		gap: var(--space-xs-sm);
		margin-left: auto;
	}
	.right-wrapper > p {
		color: var(--gray-900);
	}
</style>
