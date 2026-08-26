<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { loadSchema } from '$lib/utils/query/loadSchema';
	import type * as graph from '$lib/wails/graph';

	/**
	 * Why an expanded database has nothing under it.
	 *
	 * The alternative is an empty node, which looks like the app failing rather
	 * than the server declining. The server's own words are shown — they are what
	 * a DBA needs, and paraphrasing them would lose the part that identifies the
	 * cause.
	 */
	let { database, message }: { database: graph.DBInstanceNode; message: string } = $props();

	let retrying = $state(false);

	const retry = async () => {
		retrying = true;
		await loadSchema({ database, noCache: true });
		retrying = false;
	};
</script>

<div class="row">
	<Icon icon="cross" size={12} stroke="var(--red)" strokeWidth={3} />
	<div class="body">
		<p class="title">Could not read this database's schema</p>
		<p class="message">{message}</p>
		<Button content="Retry" size="sm" emphasis="low" onclick={retry} disabled={retrying} />
	</div>
</div>

<style>
	.row {
		display: flex;
		gap: var(--space-xs);
		padding: var(--space-xs) var(--space-sm);
		align-items: flex-start;
	}

	.body {
		display: flex;
		flex-direction: column;
		gap: var(--space-xxs);
		align-items: flex-start;
		min-width: 0;
	}

	.title {
		color: var(--gray-1000);
	}

	.message {
		color: var(--gray-800);
		word-break: break-word;
	}
</style>
