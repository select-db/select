<script lang="ts">
	import type { graph } from '$lib/wailsjs/go/models';
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
		disabled={isLoading}
		leftIcon="chart"
		iconSize={17}
		label="Analyze (execute)"
		{onclick}
	/>
{:else if plan}
	<Button
		disabled={isLoading}
		leftIcon="map"
		iconSize={18}
		label="Plan (no execution)"
		{onclick}
	/>
{:else}
	<Button
		leftIcon={isLoading ? 'pause' : 'play'}
		iconSize={17}
		{onclick}
		noLoader
		label={isLoading ? 'Cancel' : 'Run'}
	/>
{/if}
