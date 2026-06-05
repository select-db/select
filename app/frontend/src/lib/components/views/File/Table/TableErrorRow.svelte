<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';

	type Props = {
		error: string;
		onFixInChat: (error: string) => void;
		onRun: () => void;
	};
	let { error, onFixInChat, onRun }: Props = $props();
</script>

<div class="error-row">
	<p class="error-text selectable">{error}</p>
	<div class="actions">
		<Button content="Re-run" leftIcon="refresh" iconSize={16} emphasis="low" onclick={onRun} />

		<Button
			content="Fix in chat"
			leftIcon="chat"
			iconSize={16}
			emphasis="low"
			onclick={() => onFixInChat(error)}
		/>
	</div>
</div>

<style>
	.error-row {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm-md);
		align-items: flex-start;
		padding-bottom: var(--space-sm);
	}

	.error-row p {
		padding: var(--space-lg) var(--space-lg) 0 var(--space-md);
		font-weight: var(--fw-light);
	}

	.actions {
		padding-left: var(--space-sm);
		display: flex;
		gap: var(--space-xs);
		align-items: center;
	}
</style>
