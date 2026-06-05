<script lang="ts">
	import type { Component } from 'svelte';
	import { notifySuccess, notifyError } from '$lib/system/Notifications/notificationsStore';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import Button from '$lib/system/Button/Button.svelte';
	import { ResetTheme } from '$lib/wailsjs/go/system/System';
	import ConfirmResetModal from './ConfirmResetModal.svelte';

	type Props = {
		hasUnsavedChanges: boolean;
		isModifiedFromDefault: boolean;
		onSave: () => Promise<void>;
	};

	let { hasUnsavedChanges, isModifiedFromDefault, onSave }: Props = $props();

	async function handleSave() {
		const [, err] = await tryCatch(onSave);
		if (err) return notifyError(err);
		notifySuccess('Theme applied');
	}

	function handleReset() {
		modalStore.set({
			content: (() => ConfirmResetModal) as () => Component,
			props: {
				title: 'Reset theme',
				message: 'Are you sure you want to reset your theme to default values?',
				onConfirm: async () => {
					const [, err] = await tryCatch(ResetTheme);
					if (err) return notifyError(err);
					notifySuccess('Theme reset to default');
				}
			},
			width: 380
		});
	}
</script>

{#if hasUnsavedChanges || isModifiedFromDefault}
	<div class="wrapper">
		<div class="actions">
			{#if hasUnsavedChanges}
				<Button leftIcon="check" iconSize={18} content="Apply" onclick={handleSave} emphasis="high" />
			{/if}
			{#if isModifiedFromDefault}
				<Button leftIcon="refresh" iconSize={18} content="Reset" onclick={handleReset} />
			{/if}
		</div>
	</div>
{/if}

<style>
	.wrapper {
		min-height: 35px;
		background-color: var(--gray-0);
		display: flex;
		gap: var(--space-sm);
		justify-content: space-between;
		padding: var(--space-xs-sm) var(--space-sm) 0 var(--space-sm);

		border-bottom: var(--border);
	}

	.actions {
		height: 26px;
		display: flex;
		gap: var(--space-xs);
		align-items: stretch;
		padding-top: var(--space-xxs);
	}
</style>
