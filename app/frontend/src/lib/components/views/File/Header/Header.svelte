<script lang="ts">
	import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import DatasourcePicker from './DatasourcePicker.svelte';
	import RunButton from './RunButton.svelte';
	import VariablePicker from './VariablePicker.svelte';
	import { updateTab, type Tab } from '$lib/components/Layout/layoutStore';
	import type * as graph from '$lib/wails/graph';

	type Props = {
		isTemp?: boolean;
		tab: Tab;
		run: (type: 'run' | 'explain' | 'plan', datasourceIds?: string[]) => Promise<void>;
		cancel: () => Promise<void>;
		datasourcePickerOpen?: boolean;
	};

	let { isTemp, tab, run, cancel, datasourcePickerOpen = $bindable(false) }: Props = $props();

	const file = $derived.by(() => tab.file?.node);

	const selectedDatasourceIds = $derived.by(() => file?.datasources?.map((d) => d.id) ?? []);

	const onDatasourceChange = async (value: string | string[]) => {
		if (!file) return;

		const ids = Array.isArray(value) ? value : value ? [value] : [];

		const graph = $workspaceGraphStore;
		const datasources = (graph?.datasources ?? [])
			.filter((db) => ids.includes(db.id))
			.map((db) => ({ id: db.id, name: db.name }));

		let activeDatasourceId = tab.file?.activeDatasourceId;
		if (!datasources.find(({ id }) => id === activeDatasourceId))
			activeDatasourceId = datasources[0]?.id;

		updateTab({
			...tab,
			file: {
				...tab.file,
				activeDatasourceId,
				node: {
					...file,
					datasources
				} as graph.FileNode
			}
		});

		if (isTemp) return;

		await must(
			tryCatch(fs.Write, {
				uri: `${file.id}.metadata.json`,
				content: JSON.stringify({ datasources }, null, 2)
			})
		);
	};
</script>

<div class="wrapper">
	<div class="left-wrapper">
		<div class="actions-wrapper">
			<RunButton {file} {run} {cancel} />
			<RunButton {file} {run} {cancel} plan />
			<RunButton {file} {run} {cancel} explain />
		</div>
		<div class="divider"></div>
		<DatasourcePicker
			multiple={true}
			value={selectedDatasourceIds}
			onchange={onDatasourceChange}
			bind:open={datasourcePickerOpen}
		/>
	</div>

	<div class="right-wrapper">
		{#if file && !isTemp}
			<VariablePicker uri={file.uri} allowSqlFileRefs={file?.name?.endsWith('.sql') ?? false} />
		{/if}
	</div>
</div>

<style>
	.wrapper {
		background-color: var(--gray-200);

		display: flex;
		gap: var(--space-sm);
		align-items: start;
		padding: var(--space-sm) var(--space-sm-md) 0 var(--space-sm-md);
	}

	.divider {
		width: var(--space-sm);
	}

	.wrapper .left-wrapper,
	.wrapper .actions-wrapper,
	.wrapper .right-wrapper {
		display: flex;
		align-items: center;
	}

	.actions-wrapper {
		display: flex;
		gap: var(--space-xs-sm);
	}

	.wrapper .right-wrapper {
		gap: var(--space-sm);
		margin: var(--space-xxs) 0 0 auto;
	}
</style>
