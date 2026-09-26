<script lang="ts">
	import type * as graph from '$lib/wails/graph';
	import { untrack, type Component } from 'svelte';

	import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
	import {
		getTabById,
		getTabByNodeId,
		getTabUri,
		updateTab,
		removeTab,
		activeGroupStore,
		type Tab
	} from '$lib/components/Layout/layoutStore';
	import { notifyError } from '$lib/system/Notifications/notificationsStore';
	import { tryCatch, must } from '$lib/utils/tryCatch';
	import { debounce } from '$lib/utils/debounce';
	import { cancelQuery } from '$lib/utils/query/useQuery';
	import { loadingStore, toKey } from '$lib/utils/query/loadingStore';
	import {
		getDatasourceIds,
		runStatement,
		type RunStatementResult
	} from '$lib/utils/query/helpers';
	import { registerCommand, unregisterCommand } from '$lib/stores/commandRegistry';
	import * as graphApi from '$lib/bindings/selectDb/internal/graph/graph';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import RuntimeVarsModal from '../RuntimeVarsModal.svelte';

	import Editor from '../Editor/Editor.svelte';
	import Table from '../Table/Table.svelte';
	import Header from '../Header/Header.svelte';
	import TableHeader from '../Table/TableHeader/TableHeader.svelte';
	import {
		getEffectiveSelectedDatasourceId,
		getQueryResultForDatasource,
		getPlanResultForDatasource,
		getExplainResultForDatasource
	} from './tableViewState';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	const file = $derived(tab.file?.node);
	const isTemp = $derived(tab.file?.isTemp ?? false);
	let content = $state<string>('');
	let contentLoaded = $state(false);
	let tableHeight = $derived(tab.file?.tableHeight ?? 0);

	let datasourcePickerOpen = $state(false);
	let wasDatasourcePickerOpen = $state(false);

	$effect(() => {
		// Focus the editor after closing DB picker
		const justClosed = !datasourcePickerOpen && wasDatasourcePickerOpen;
		wasDatasourcePickerOpen = datasourcePickerOpen;
		if (!justClosed) return;

		queueMicrotask(() => editorRef?.focus());
	});

	$effect(() => {
		if (!tab.file) return;
		if (tab.file.tableHeight === tableHeight) return;

		updateTab({
			...tab,
			file: {
				...tab.file,
				tableHeight
			}
		});
	});

	let currentLoadingTabId: string | undefined;
	$effect(() => {
		const tabId = tab.id;
		const currentFile = tab.file;

		const alreadyHandled = untrack(() => currentLoadingTabId === tabId);
		if (alreadyHandled) return;

		currentLoadingTabId = tabId;

		if (!currentFile?.node) {
			content = '';
			contentLoaded = false;
			return;
		}

		if (currentFile.isTemp) {
			content = currentFile.content ?? '';
			contentLoaded = true;
			return;
		}

		content = '';
		contentLoaded = false;
		readFileContent(currentFile.node.uri);
	});

	async function readFileContent(uri: string) {
		const [fileContent, err] = await tryCatch(fs.ReadFile, { uri });
		if (err) {
			notifyError(`Failed to read file: ${uri}`);
			removeTab(tab.id);
			return;
		}

		content = fileContent ?? '';
		contentLoaded = true;
	}

	// The path is looked up when the write goes out, not when the key was
	// pressed: a rename in between moves the file, and a write to where it used
	// to be recreates it there -- an old folder coming back from the dead,
	// holding the text that belongs to the new one. The tab id is what survives
	// the rename, so that is what is remembered.
	const writeFile = async (tabId: string, content: string) => {
		const uri = getTabUri(tabId);
		if (!uri) return;
		await must(tryCatch(fs.Write, { uri, content }));
	};

	const writeToFile = debounce(writeFile, 200);

	// What the editor was holding for a tab it is being taken off, which is the
	// one moment a debounced write would be too late: it is addressed by tab id
	// because the tab is no longer the one on screen.
	const savePendingChange = (tabId: string, pending: string) => {
		const target = getTabById(tabId);
		if (!target?.file) return;

		if (target.file.isTemp) {
			updateTab({ ...target, file: { ...target.file, content: pending } });
			return;
		}
		void writeFile(tabId, pending);
	};

	const handleContentChange = (newContent: string) => {
		content = newContent;

		if (isTemp) {
			updateTab({
				...tab,
				file: {
					...tab.file!,
					content
				}
			});
			return;
		}

		if (file) {
			writeToFile(tab.id, content);
		}
	};

	const datasourceIds = $derived(getDatasourceIds(file ?? null));

	const cancel = async () => {
		if (!file || datasourceIds.length === 0) return;
		for (const datasourceId of datasourceIds) {
			await must(
				tryCatch(cancelQuery, {
					FileID: file.id,
					DatasourceID: datasourceId
				})
			);
		}
	};

	const varPattern = /\$([A-Za-z_][A-Za-z0-9_]*)/g;

	const persistRuntimeVars = debounce(
		(vals: Record<string, string>, types: Record<string, string>) => {
			const currentTab = getTabByNodeId(file?.id ?? '');
			if (!currentTab?.file) return;
			updateTab({
				...currentTab,
				file: {
					...currentTab.file,
					runtimeVars: { ...(currentTab.file.runtimeVars ?? {}), ...vals },
					runtimeVarTypes: { ...(currentTab.file.runtimeVarTypes ?? {}), ...types }
				}
			});
		},
		300
	);

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
					onValuesChange: (vals: Record<string, string>, types: Record<string, string>) => {
						persistRuntimeVars(vals, types);
					},
					onSubmit: (vals: Record<string, string>, types: Record<string, string>) => {
						modalStore.set(null);
						queueMicrotask(() => editorRef?.focus());
						resolve({ vals, types });
					},
					onCancel: () => {
						modalStore.set(null);
						queueMicrotask(() => editorRef?.focus());
						resolve(null);
					}
				},
				width: 440
			});
		});

	/**
	 * Drops the last result for the databases about to run.
	 *
	 * A run only writes its own result once `query:started` comes back, which on
	 * a remote or tunnelled connection is seconds away. Until then the pane kept
	 * showing the previous run's rows, row count and duration — and because a
	 * finished duration stops the clock, the header read as a query that had
	 * already returned while it was still going. The pane already has a running
	 * state; it just could not reach it while the old result sat in the slot.
	 */
	const clearResultsForRun = (datasourceIdsToClear: string[], mode: 'run' | 'explain' | 'plan') => {
		if (!file) return;

		const currentTab = getTabByNodeId(file.id);
		if (!currentTab?.file?.node) return;

		const key =
			mode === 'run' ? 'queryResults' : mode === 'plan' ? 'planResults' : 'explainResults';
		const cleared = { ...(currentTab.file.node[key] ?? {}) };
		for (const datasourceId of datasourceIdsToClear) delete cleared[datasourceId];

		updateTab({
			...currentTab,
			file: {
				...currentTab.file,
				node: { ...currentTab.file.node, [key]: cleared } as graph.FileNode
			}
		});
	};

	const run = async (mode: 'run' | 'explain' | 'plan', datasourceIdsArg?: string[]) => {
		if (!file || !tab.file || !tab.file.node) return;

		const targetDatasourceIds = datasourceIdsArg ?? datasourceIds;
		if (targetDatasourceIds.length === 0) {
			notifyError('Select a database before running this file');
			return;
		}

		const activeDatasourceId = datasourceIdsArg?.length === 1 ? datasourceIdsArg[0] : undefined;

		if (targetDatasourceIds.some((id) => $loadingStore.includes(toKey(id, file.id)))) {
			await cancel();
			return;
		}

		// Detect unresolved $variables using existing GetUriVariables RPC
		const folderId = (file as graph.FileNode & { folder_id?: string }).folder_id ?? '';
		const [resolvedVars] = await tryCatch(graphApi.GetUriVariables, file.uri);
		const resolvedNames = new Set((resolvedVars ?? []).map((v) => v.name));
		const allVarNames = [...new Set([...content.matchAll(varPattern)].map((m) => m[1]))];
		const unresolvedNames = allVarNames.filter((name) => !resolvedNames.has(name));

		let runtimeVars: Record<string, string> = { ...(tab.file.runtimeVars ?? {}) };
		if (unresolvedNames.length > 0) {
			// Always prompt, stored values pre-fill the form
			const result = await promptRuntimeVars(
				unresolvedNames,
				runtimeVars,
				tab.file.runtimeVarTypes ?? {}
			);
			if (result === null) return; // user cancelled
			runtimeVars = { ...runtimeVars, ...result.vals };
			// Persist values and types to tab store
			const currentTab = getTabByNodeId(file.id);
			if (currentTab?.file) {
				updateTab({
					...currentTab,
					file: {
						...currentTab.file,
						runtimeVars,
						runtimeVarTypes: { ...(currentTab.file.runtimeVarTypes ?? {}), ...result.types }
					}
				});
			}
		}

		clearResultsForRun(targetDatasourceIds, mode);

		const results: Record<string, RunStatementResult> = {};
		for (const datasourceId of targetDatasourceIds) {
			const result = await runStatement({
				statement: content,
				datasourceId: datasourceId,
				fileId: file.id,
				folderId,
				explain: mode === 'explain',
				plan: mode === 'plan',
				runtimeVars
			});
			if (result) results[datasourceId] = result;
		}

		const currentTab = getTabByNodeId(file.id);
		if (!currentTab || !currentTab.file) return;

		let tables = currentTab.file.tables ?? {};
		if (mode === 'run') {
			for (const datasourceId of targetDatasourceIds) {
				tables = {
					...tables,
					[datasourceId]: {
						...(tables[datasourceId] ?? {}),
						edits: {},
						scrollLeft: 0,
						scrollTop: 0
					}
				};
			}
		}

		const queryResults: NonNullable<graph.FileNode['queryResults']> = {
			...currentTab.file.node.queryResults
		};
		const planResults: NonNullable<graph.FileNode['planResults']> = {
			...(currentTab.file.node.planResults ?? {})
		};
		const explainResults: NonNullable<graph.FileNode['explainResults']> = {
			...currentTab.file.node.explainResults
		};

		for (const [datasourceId, res] of Object.entries(results)) {
			if (res.type === 'query') {
				queryResults[datasourceId] = res.result;
			} else if (mode === 'plan') {
				planResults[datasourceId] = res.result;
			} else if (mode === 'explain') {
				explainResults[datasourceId] = res.result;
			}
		}

		updateTab({
			...currentTab,
			file: {
				...currentTab.file,
				node: {
					...currentTab.file.node,
					queryResults,
					planResults,
					explainResults
				} as graph.FileNode,
				viewMode:
					mode === 'run' ? (currentTab.file.viewMode === 'graph' ? 'graph' : 'results') : mode,
				...(activeDatasourceId != null && { activeDatasourceId: activeDatasourceId }),
				tables
			}
		});
	};

	// Tabs opened with a prefilled statement (e.g. "View data" on a table) run it
	// once, as soon as the content is in the editor. This view instance is shared
	// by every tab of its group (see the content loader above), so the guard
	// tracks the tab it ran for rather than being a one-shot flag. Clearing the
	// tab's own flag then keeps the query from replaying when the tab is
	// re-selected later.
	let autoRunTabId: string | undefined;
	$effect(() => {
		if (!contentLoaded) return;
		if (!tab.file?.runOnOpen) return;
		if (autoRunTabId === tab.id) return;

		autoRunTabId = tab.id;
		updateTab({ ...tab, file: { ...tab.file, runOnOpen: false } });
		void run('run');
	});

	const effectiveDatasourceId = $derived(getEffectiveSelectedDatasourceId(file, tab));
	const currentQueryResult = $derived(getQueryResultForDatasource(file, effectiveDatasourceId));
	const currentPlanResult = $derived(getPlanResultForDatasource(file, effectiveDatasourceId));
	const currentExplainResult = $derived(getExplainResultForDatasource(file, effectiveDatasourceId));

	$effect(() => {
		if (!file || !tab.file) return;

		const datasourceId = effectiveDatasourceId;
		if (!datasourceId) return;

		const hasQueryResult = currentQueryResult?.id;
		const hasPlanResult = currentPlanResult?.root;
		const hasExplainResult = currentExplainResult?.root;
		if (!hasQueryResult && !hasPlanResult && !hasExplainResult) return;

		const activeView = tab.file.viewMode ?? 'results';
		const currentResultId =
			activeView === 'plan'
				? (currentPlanResult?.id ?? null)
				: activeView === 'explain'
					? (currentExplainResult?.id ?? null)
					: (currentQueryResult?.id ?? null);
		if (!currentResultId) return;

		const prevResultId = tab.file.tables?.[datasourceId]?.lastResultId ?? null;

		if (currentResultId === prevResultId) return;

		const shouldOpen = untrack(() => tableHeight === 0);
		if (shouldOpen) tableHeight = 195;

		updateTab({
			...tab,
			file: {
				...tab.file,
				...(shouldOpen && { tableHeight: 195 }),
				tables: {
					...(tab.file.tables ?? {}),
					[datasourceId]: {
						...(tab.file.tables?.[datasourceId] ?? {}),
						lastResultId: currentResultId
					}
				}
			}
		});
	});

	let editorRef = $state<ReturnType<typeof Editor> | undefined>();

	const isFocused = $derived($activeGroupStore?.activeTabId === tab.id);
	$effect(() => {
		// Register editor commands only while this tab is the focused one (active tab in active group).
		if (!isFocused) return;

		const runHandler = () => {
			const viewMode = tab.file?.viewMode ?? 'run';
			run(viewMode === 'results' || viewMode === 'graph' ? 'run' : viewMode);
		};
		const formatHandler = () => editorRef?.format();
		const datasourcePickerHandler = () => {
			datasourcePickerOpen = !datasourcePickerOpen;
		};

		registerCommand('editor.runQuery', runHandler);
		registerCommand('editor.formatDocument', formatHandler);
		registerCommand('editor.toggleDatasourcePicker', datasourcePickerHandler);

		return () => {
			unregisterCommand('editor.runQuery', runHandler);
			unregisterCommand('editor.formatDocument', formatHandler);
			unregisterCommand('editor.toggleDatasourcePicker', datasourcePickerHandler);
		};
	});
</script>

<div class="sql-view">
	{#if file}
		<Header {tab} {isTemp} {run} {cancel} bind:datasourcePickerOpen />
	{/if}

	<div class="page">
		{#if contentLoaded}
			<Editor
				bind:this={editorRef}
				{tab}
				{content}
				language="sql-custom"
				onContentChange={handleContentChange}
				onPendingChange={savePendingChange}
				errorPosition={currentQueryResult?.errors?.length
					? (currentQueryResult?.errorPosition ?? undefined)
					: undefined}
				errorMessage={currentQueryResult?.errors?.[0] ?? undefined}
			/>
		{/if}
	</div>

	<div class="table-section">
		<TableHeader {tab} {run} {content} bind:tableHeight />
		<Table
			{tableHeight}
			{tab}
			{effectiveDatasourceId}
			{currentQueryResult}
			{currentPlanResult}
			{currentExplainResult}
			{run}
			{content}
		/>
	</div>
</div>

<style>
	.sql-view {
		height: 100%;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		background-color: var(--gray-200);
	}

	.page {
		display: flex;
		overflow: hidden;
		flex: 1 1 0;
		min-height: 0;
	}

	.table-section {
		flex-shrink: 0;
	}

	.page :global(Editor) {
		flex: 1 1 0;
		min-width: 0;
		min-height: 0;
	}
</style>
