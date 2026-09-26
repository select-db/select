<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { updateTab } from '$lib/components/Layout/layoutStore';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { loadSchema, loadSchemaIfEmpty } from '$lib/utils/query/loadSchema';
	import { getTableDDL, getSchemaTableOptionGroups } from './getTableDDL';
	import SqlViewer from '$lib/system/SqlViewer/SqlViewer.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Select from '$lib/system/Select/Select.svelte';
	import type { SelectOption } from '$lib/system/Select/Select.types';
	import DatasourcePicker from '$lib/components/views/File/Header/DatasourcePicker.svelte';
	import { tryCatch } from '$lib/utils/tryCatch';
	import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
	import Icon from '$lib/system/Icon/Icon.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	let schemaContent = $state('');
	let schemaLoading = $state(false);

	const datasourceId = $derived(tab.schema?.datasourceId);
	const selectedSchemaTable = $derived(tab.schema?.selectedSchemaTable ?? '');

	const datasource = $derived(
		($workspaceGraphStore?.datasources ?? []).find((dbi) => dbi.id === datasourceId)
	);

	const schemaFileUri = $derived(
		datasource?.uri ? `${datasource.uri.replace(/\/$/, '')}/schema.sql` : null
	);

	const schemaTableOptionGroups = $derived(
		datasourceId && datasource?.children?.length
			? [
					{ label: 'Full schema', options: [{ value: '', label: 'Full schema' }] },
					...getSchemaTableOptionGroups($workspaceGraphStore, datasourceId)
				]
			: []
	);

	const displayContent = $derived(
		selectedSchemaTable && datasourceId
			? getTableDDL($workspaceGraphStore, datasourceId, selectedSchemaTable)
			: null
	);
	const effectiveContent = $derived(
		selectedSchemaTable && datasourceId ? (displayContent ?? schemaContent) : schemaContent
	);

	async function readSchemaFile() {
		const uri = schemaFileUri;
		if (!uri) return;
		schemaLoading = true;
		const [content, err] = await tryCatch(fs.ReadFile, { uri });
		schemaLoading = false;
		if (schemaFileUri !== uri) return;
		schemaContent = err ? 'no_schema' : (content ?? 'no_schema');
	}

	$effect(() => {
		if (!datasourceId) {
			const firstDb = $workspaceGraphStore?.datasources?.[0];
			if (firstDb) {
				updateTab({
					...tab,
					schema: { ...tab.schema, datasourceId: firstDb.id, datasourceName: firstDb.name }
				});
			}
			return;
		}

		if (!datasource) return;
		// loadSchemaIfEmpty returns at once when the schema is already there, so
		// the file is read either way and only this knows when.
		void loadSchemaIfEmpty(datasource).then(() => readSchemaFile());
	});

	const onDatasourceChange = (value: string | string[]) => {
		const newDatasourceId = Array.isArray(value) ? (value[0] ?? '') : value;
		const newDatasource = ($workspaceGraphStore?.datasources ?? []).find(
			(dbi) => dbi.id === newDatasourceId
		);
		updateTab({
			...tab,
			schema: {
				...tab.schema,
				datasourceId: newDatasourceId,
				datasourceName: newDatasource?.name,
				selectedSchemaTable: undefined
			}
		});
	};

	let sqlViewer: { scrollToTop: () => void } | null = $state(null);

	const onSchemaTableChange = (value: string | string[]) => {
		const v = Array.isArray(value) ? (value[0] ?? '') : value;
		updateTab({
			...tab,
			schema: { ...tab.schema, selectedSchemaTable: v }
		});
		sqlViewer?.scrollToTop();
	};

	const onRefresh = async () => {
		if (!datasource) return;
		await loadSchema({ datasource: datasource });
		await readSchemaFile();
	};
</script>

<div class="schema-view">
	<div class="header">
		<div class="wrapper">
			<Button
				leftIcon="refresh"
				size="sm"
				emphasis="low"
				iconSize={18}
				onclick={onRefresh}
				disabled={!datasource}
				label="Refresh Schema"
			/>
			<div class="picker-wrapper">
				<DatasourcePicker value={datasourceId} onchange={onDatasourceChange} />
				{#if datasourceId && schemaTableOptionGroups.length > 0}
					<Icon icon="chevron-right" size={18} />
					<div style="width: var(--space-md)"></div>
					<Select
						optionGroups={schemaTableOptionGroups}
						value={selectedSchemaTable}
						onchange={onSchemaTableChange}
						placeholder="Full schema"
						searchEnabled={true}
						searchPlaceholder="Search table..."
						menuWidth={275}
						size="xs"
						emphasis="low"
					>
						{#snippet optionDisplay(option: SelectOption<string> | null)}
							{#if option}
								<span class="table-option">
									{#if option.value !== ''}
										<Icon icon="table" size={16} stroke="var(--gray-800)" />
									{/if}
									<span class="table-option-label">{option.label}</span>
								</span>
							{:else}
								<span class="table-option-placeholder">Full schema</span>
							{/if}
						{/snippet}
					</Select>
				{/if}
			</div>
		</div>
	</div>

	<div class="content">
		{#if !datasourceId}
			<div class="empty-state">
				<p>Select a database to view its schema</p>
			</div>
		{:else if schemaLoading}
			<div class="empty-state">
				<p>Loading schema…</p>
			</div>
		{:else if schemaContent === 'no_schema'}
			<div class="empty-state">
				<p>No schema available</p>
			</div>
		{:else}
			<SqlViewer bind:this={sqlViewer} sql={effectiveContent} className="surface-light" />
		{/if}
	</div>
</div>

<style>
	.schema-view {
		display: flex;
		flex-direction: column;

		height: 100%;
	}

	.header {
		display: flex;
		align-items: start;

		padding: var(--space-xs-sm) var(--space-sm) var(--space-sm) var(--space-sm);
		border-bottom: var(--border);
	}

	.wrapper {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
	}

	.picker-wrapper {
		display: flex;
	}

	.table-option {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		min-width: 0;
		width: 100%;
	}

	.table-option-label {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.table-option-placeholder {
		color: var(--gray-800);
	}

	.content {
		flex: 1;
		overflow: auto;
	}

	.empty-state {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: var(--gray-800);
	}
</style>
