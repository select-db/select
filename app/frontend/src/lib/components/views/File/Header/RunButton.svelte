<script lang="ts">
	import type * as graph from '$lib/wails/graph';
	import Button from '$lib/system/Button/Button.svelte';
	import { getDbIds } from '$lib/utils/query/helpers';
	import { loadingStore, toKey } from '$lib/utils/query/loadingStore';

	type RunButtonProps = {
		file: graph.FileNode | null | undefined;
		run: (type: 'run' | 'explain' | 'plan', dbIds?: string[]) => Promise<void>;
		cancel: () => Promise<void>;
		explain?: boolean;
		plan?: boolean;
	};

	let { file, run, cancel, explain, plan }: RunButtonProps = $props();

	const dbIds = $derived(getDbIds(file ?? null));
	const isLoading = $derived(dbIds.some((id) => $loadingStore.includes(toKey(id, file?.id))));

	const onclick = async () => {
		if (isLoading) {
			await cancel();
			return;
		}
		const type = explain ? 'explain' : plan ? 'plan' : 'run';
		await run(type);
	};
</script>

{#if explain}
	<Button
		emphasis="high"
		disabled={isLoading}
		leftIcon="chart"
		iconSize={16}
		label="Analyze (execute)"
		classes="sql-action-btn"
		{onclick}
	/>
{:else if plan}
	<Button
		emphasis="high"
		disabled={isLoading}
		leftIcon="map"
		iconSize={17}
		label="Plan (no execution)"
		classes="sql-action-btn"
		{onclick}
	/>
{:else}
	<Button
		emphasis="high"
		leftIcon={isLoading ? 'pause' : 'play'}
		iconSize={16}
		label={isLoading ? 'Cancel' : 'Run'}
		classes="sql-action-btn"
		{onclick}
		noLoader
	/>
{/if}

<style>
	:global(button.sql-action-btn) {
		margin-left: var(--space-xxs);
	}
</style>
