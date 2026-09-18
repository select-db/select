<script lang="ts">
	type Props = {
		status: string | null;
	};

	let { status }: Props = $props();
</script>

{#if status}
	<p class="status-indicator" aria-live="polite">{status}</p>
{/if}

<style>
	.status-indicator {
		color: var(--gray-800);
		align-self: flex-start;
		font-size: var(--fs-sm);
		margin: 0 var(--space-sm-md);
	}
</style>
