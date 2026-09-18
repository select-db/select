<script lang="ts">
	import Button, { type ButtonEmphasis } from '../Button/Button.svelte';

	type ActionProps = {
		label: string;
		action: () => Promise<void>;
		disabled?: boolean;
		/** "warning" for anything destructive. Defaults to the button's place in
		 * the footer: the main action is high, the others low. */
		emphasis?: ButtonEmphasis;
	};

	type ModalFooterProps = {
		mainAction?: ActionProps;
		secondaryAction?: ActionProps;
		leftAction?: ActionProps;
	};

	let { mainAction, secondaryAction, leftAction }: ModalFooterProps = $props();
</script>

<div class="footer" class:has-left={!!leftAction}>
	{#if leftAction}
		<Button
			content={leftAction.label}
			onclick={leftAction.action}
			disabled={leftAction.disabled}
			emphasis={leftAction.emphasis ?? 'low'}
		/>
	{/if}
	<div class="main-group">
		{#if secondaryAction}
			<Button
				content={secondaryAction.label}
				onclick={secondaryAction.action}
				disabled={secondaryAction.disabled}
				emphasis={secondaryAction.emphasis ?? 'low'}
			/>
		{/if}
		{#if mainAction}
			<Button
				content={mainAction.label}
				onclick={mainAction.action}
				disabled={mainAction.disabled}
				emphasis={mainAction.emphasis ?? 'high'}
			/>
		{/if}
	</div>
</div>

<style>
	.footer {
		background-color: var(--gray-200);
		padding: var(--space-sm);
		border-top: var(--border);

		display: flex;
		justify-content: flex-end;
		gap: var(--space-sm);
	}

	.footer.has-left {
		justify-content: space-between;
	}

	.main-group {
		display: flex;
		gap: var(--space-sm);
	}
</style>
