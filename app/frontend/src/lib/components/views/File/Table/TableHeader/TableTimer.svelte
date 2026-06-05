<script lang="ts">
	import { onDestroy } from 'svelte';

	import { formatMs } from '$lib/utils/formatMs';

	type Props = {
		loading?: boolean;
		durationMs?: number | null;
	};

	let { loading = false, durationMs }: Props = $props();

	let elapsedMs = $state(0);
	let timer: ReturnType<typeof setInterval> | null = null;

	const hasFinalDuration = $derived(durationMs != null && durationMs > 0);

	$effect(() => {
		// Stop ticking as soon as the engine surfaces a real durationMs, even
		// if rows are still streaming. The clock keeps the SQL execution time,
		// not the row-delivery time.
		if (loading && !hasFinalDuration) {
			elapsedMs = 0;
			clearInterval(timer!);
			timer = setInterval(() => (elapsedMs += 37), 37);
		} else {
			clearInterval(timer!);
			timer = null;
		}
	});

	onDestroy(() => {
		if (!timer) return;

		clearInterval(timer);
	});
</script>

<div class="wrapper">
	{#if hasFinalDuration}
		<p>{formatMs(durationMs ?? 0)}</p>
	{:else if loading}
		<p>{formatMs(elapsedMs)}</p>
	{:else}
		<p class="muted">00:00.000</p>
	{/if}
</div>

<style>
	.wrapper {
		display: flex;
		gap: var(--space-xs);
		padding-right: var(--space-sm);
		align-items: center;
		width: 60px;
	}
	.muted {
		color: var(--gray-800);
	}
</style>
