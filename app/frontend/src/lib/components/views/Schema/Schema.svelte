<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { updateTab } from '$lib/components/Layout/layoutStore';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { loadSchema } from '$lib/utils/query/loadSchema';
	import { getTableDDL, getSchemaTableOptionGroups } from './getTableDDL';
	import SqlViewer from '$lib/system/SqlViewer/SqlViewer.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Select from '$lib/system/Select/Select.svelte';
	import type { SelectOption } from '$lib/system/Select/Select.types';
	import DatabasePicker from '$lib/components/views/File/Header/DatabasePicker.svelte';
	import { tryCatch } from '$lib/utils/tryCatch';
	import * as fs from '$lib/wailsjs/go/fs_provider/FSProvider';
	import Icon from '$lib/system/Icon/Icon.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	let schemaContent = $state('');
	let schemaLoading = $state(false);

	const databaseId = $derived(tab.schema?.databaseId);
	const selectedSchemaTable = $derived(tab.schema?.selectedSchemaTable ?? '');

	const dbInstance = $derived(
		$workspaceGraphStore?.db_instances?.find((dbi) => dbi.id === databaseId)
	);

	const schemaFileUri = $derived(
		dbInstance?.uri ? `${dbInstance.uri.replace(/\/$/, '')}/schema.sql` : null
	);

	const schemaTableOptionGroups = $derived(
		databaseId && dbInstance?.children?.length
			? [
					{ label: 'Full schema', options: [{ value: '', label: 'Full schema' }] },
					...getSchemaTableOptionGroups($workspaceGraphStore, databaseId)
				]
			: []
	);

	const displayContent = $derived(
		selectedSchemaTable && databaseId
			? getTableDDL($workspaceGraphStore, databaseId, selectedSchemaTable)
			: null
	);
	const effectiveContent = $derived(
		selectedSchemaTable && databaseId ? (displayContent ?? schemaContent) : schemaContent
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
		if (!databaseId) {
			const firstDb = $workspaceGraphStore?.db_instances?.[0];
			if (firstDb) {
				updateTab({
					...tab,
					schema: { ...tab.schema, databaseId: firstDb.id, databaseName: firstDb.name }
				});
			}
			return;
		}

		if (!dbInstance) return;
		const hasSchema = dbInstance.children && dbInstance.children.length > 0;
		if (!hasSchema) {
			void loadSchema({ database: dbInstance }).then(() => readSchemaFile());
		} else {
			readSchemaFile();
		}
	});

	const onDatabaseChange = (value: string | string[]) => {
		const newDatabaseId = Array.isArray(value) ? (value[0] ?? '') : value;
		const newDbInstance = $workspaceGraphStore?.db_instances?.find(
			(dbi) => dbi.id === newDatabaseId
		);
		updateTab({
			...tab,
			schema: {
				...tab.schema,
				databaseId: newDatabaseId,
				databaseName: newDbInstance?.name,
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
		if (!dbInstance) return;
		await loadSchema({ database: dbInstance, noCache: true });
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
				disabled={!dbInstance}
				label="Refresh Schema"
			/>
			<div class="picker-wrapper">
				<DatabasePicker value={databaseId} onchange={onDatabaseChange} />
				{#if databaseId && schemaTableOptionGroups.length > 0}
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
		{#if !databaseId}
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
