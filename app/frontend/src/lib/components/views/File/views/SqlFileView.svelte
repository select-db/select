<script lang="ts">
	import type * as graph from '$lib/wails/graph';
	import { untrack, type Component } from 'svelte';

	import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
	import {
		getTabByNodeId,
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
	import { getDbIds, runStatement, type RunStatementResult } from '$lib/utils/query/helpers';
	import { registerCommand, unregisterCommand } from '$lib/stores/commandRegistry';
	import * as graphApi from '$lib/bindings/selectDb/internal/graph/graph';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import RuntimeVarsModal from '../RuntimeVarsModal.svelte';

	import Editor from '../Editor/Editor.svelte';
	import Table from '../Table/Table.svelte';
	import Header from '../Header/Header.svelte';
	import TableHeader from '../Table/TableHeader/TableHeader.svelte';
	import {
		getEffectiveSelectedDbId,
		getQueryResultForDb,
		getPlanResultForDb,
		getExplainResultForDb
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

	let databasePickerOpen = $state(false);
	let wasDatabasePickerOpen = $state(false);

	$effect(() => {
		// Focus the editor after closing DB picker
		const justClosed = !databasePickerOpen && wasDatabasePickerOpen;
		wasDatabasePickerOpen = databasePickerOpen;
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

	const writeToFile = debounce(async (uri: string, content: string) => {
		await must(tryCatch(fs.Write, { uri, content }));
	}, 200);

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
			writeToFile(file.uri, content);
		}
	};

	const dbIds = $derived(getDbIds(file ?? null));

	const cancel = async () => {
		if (!file || dbIds.length === 0) return;
		for (const dbId of dbIds) {
			await must(
				tryCatch(cancelQuery, {
					FileID: file.id,
					DbInstanceID: dbId
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

	const run = async (mode: 'run' | 'explain' | 'plan', dbIdsArg?: string[]) => {
		if (!file || !tab.file || !tab.file.node) return;

		const targetDbIds = dbIdsArg ?? dbIds;
		if (targetDbIds.length === 0) {
			notifyError('Select a database before running this file');
			return;
		}

		const activeDbId = dbIdsArg?.length === 1 ? dbIdsArg[0] : undefined;

		if (targetDbIds.some((id) => $loadingStore.includes(toKey(id, file.id)))) {
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

		const results: Record<string, RunStatementResult> = {};
		for (const dbId of targetDbIds) {
			const result = await runStatement({
				statement: content,
				dbInstanceId: dbId,
				fileId: file.id,
				folderId,
				explain: mode === 'explain',
				plan: mode === 'plan',
				runtimeVars
			});
			if (result) results[dbId] = result;
		}

		const currentTab = getTabByNodeId(file.id);
		if (!currentTab || !currentTab.file) return;

		let tables = currentTab.file.tables ?? {};
		if (mode === 'run') {
			for (const dbId of targetDbIds) {
				tables = {
					...tables,
					[dbId]: { ...(tables[dbId] ?? {}), edits: {}, scrollLeft: 0, scrollTop: 0 }
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

		for (const [dbId, res] of Object.entries(results)) {
			if (res.type === 'query') {
				queryResults[dbId] = res.result;
			} else if (mode === 'plan') {
				planResults[dbId] = res.result;
			} else if (mode === 'explain') {
				explainResults[dbId] = res.result;
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
				...(activeDbId != null && { activeDatabaseId: activeDbId }),
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

	const effectiveDbId = $derived(getEffectiveSelectedDbId(file, tab));
	const currentQueryResult = $derived(getQueryResultForDb(file, effectiveDbId));
	const currentPlanResult = $derived(getPlanResultForDb(file, effectiveDbId));
	const currentExplainResult = $derived(getExplainResultForDb(file, effectiveDbId));

	$effect(() => {
		if (!file || !tab.file) return;

		const dbId = effectiveDbId;
		if (!dbId) return;

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

		const prevResultId = tab.file.tables?.[dbId]?.lastResultId ?? null;

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
					[dbId]: {
						...(tab.file.tables?.[dbId] ?? {}),
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
		const dbPickerHandler = () => {
			databasePickerOpen = !databasePickerOpen;
		};

		registerCommand('editor.runQuery', runHandler);
		registerCommand('editor.formatDocument', formatHandler);
		registerCommand('editor.toggleDatabasePicker', dbPickerHandler);

		return () => {
			unregisterCommand('editor.runQuery', runHandler);
			unregisterCommand('editor.formatDocument', formatHandler);
			unregisterCommand('editor.toggleDatabasePicker', dbPickerHandler);
		};
	});
</script>

<div class="sql-view">
	{#if file}
		<Header {tab} {isTemp} {run} {cancel} bind:databasePickerOpen />
	{/if}

	<div class="page">
		{#if contentLoaded}
			<Editor
				bind:this={editorRef}
				{tab}
				{content}
				language="sql-custom"
				onContentChange={handleContentChange}
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
			{effectiveDbId}
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
