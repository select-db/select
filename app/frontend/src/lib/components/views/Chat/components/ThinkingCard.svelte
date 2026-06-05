<script lang="ts">
	type Props = {
		content: string;
		expanded: boolean;
		onToggle: () => void;
	};

	let { content, expanded, onToggle }: Props = $props();
</script>

<div class="thinking-card">
	<button type="button" class="thinking-header" onclick={onToggle} aria-expanded={expanded}>
		<span class="thinking-label">Reasoning</span>
	</button>
	{#if expanded}
		<div class="thinking-body">{content}</div>
	{/if}
</div>

<style>
	.thinking-card {
		border: var(--border);
		border-radius: var(--br-xs);
		background-color: var(--gray-0);
		overflow: hidden;
		font-size: var(--fs-sm);
	}

	.thinking-header {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		width: 100%;
		padding: var(--space-sm) var(--space-sm-md);
		background: none;
		border: none;
		font: inherit;
		color: inherit;
		text-align: left;
		outline: none;
	}

	.thinking-header:hover {
		background-color: var(--gray-400);
	}

	.thinking-label {
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--gray-800);
	}

	.thinking-body {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding: var(--space-sm-md);
		border-top: var(--border);
	}
</style>
