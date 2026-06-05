<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';

	type Props = {
		message: string;
		button: {
			onclick: () => void;
		};
	};
	let { message, button }: Props = $props();
</script>

<div class="empty-row">
	<p class="empty-text">{message}</p>
	<div class="actions">
		<Button {...button} content="run" leftIcon="play" iconSize={16} emphasis="low" />
	</div>
</div>

<style>
	.empty-row {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm-md);
		align-items: flex-start;
	}

	.empty-row p {
		padding: var(--space-lg) var(--space-lg) 0 var(--space-md);
		font-weight: var(--fw-light);
		color: var(--gray-800);
	}

	.actions {
		padding-left: var(--space-sm);
		display: flex;
		gap: var(--space-xs);
		align-items: center;
	}
</style>
