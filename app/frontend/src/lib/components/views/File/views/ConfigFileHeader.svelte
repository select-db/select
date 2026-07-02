<script lang="ts">
	import type { Component } from 'svelte';
	import { ResetUserConfig } from '$lib/wailsjs/go/system/System';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { notifySuccess, notifyError } from '$lib/system/Notifications/notificationsStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import Button from '$lib/system/Button/Button.svelte';
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
		notifySuccess('Config applied');
	}

	function handleReset() {
		modalStore.set({
			content: (() => ConfirmResetModal) as () => Component,
			props: {
				title: 'Reset config',
				message: 'Are you sure you want to reset your configuration to default values?',
				onConfirm: async () => {
					const [, err] = await tryCatch(ResetUserConfig);
					if (err) return notifyError(err);
					notifySuccess('Config reset to default');
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
				<Button
					leftIcon="check"
					iconSize={16}
					content="Apply"
					onclick={handleSave}
					emphasis="high"
				/>
			{/if}
			{#if isModifiedFromDefault}
				<Button leftIcon="refresh" iconSize={16} content="Reset" onclick={handleReset} />
			{/if}
		</div>
	</div>
{/if}

<style>
	.wrapper {
		min-height: 35px;
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
