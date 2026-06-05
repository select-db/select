<script lang="ts">
	import Button from '../Button/Button.svelte';

	type ActionProps = {
		label: string;
		action: () => Promise<void>;
		disabled?: boolean;
	};

	type ModalFooterProps = {
		mainAction: ActionProps;
		secondaryAction?: ActionProps;
	};

	let { mainAction, secondaryAction }: ModalFooterProps = $props();
</script>

<div class="footer">
	{#if secondaryAction}
		<Button
			content={secondaryAction.label}
			onclick={secondaryAction.action}
			disabled={secondaryAction.disabled}
			emphasis="low"
		/>
	{/if}
	<Button
		content={mainAction.label}
		onclick={mainAction.action}
		disabled={mainAction.disabled}
		emphasis="high"
	/>
</div>

<style>
	.footer {
		background-color: var(--gray-0);
		padding: var(--space-sm);
		border-top: var(--border);

		display: flex;
		justify-content: flex-end;
		gap: var(--space-sm);
	}
</style>
