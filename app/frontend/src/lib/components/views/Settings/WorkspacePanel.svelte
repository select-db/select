<script lang="ts">
	import type { Component } from 'svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { UpdateName, DeleteWorkspace } from '$lib/wailsjs/go/workspace/Workspace';
	import { Logout } from '$lib/wailsjs/go/system/System';
	import { get } from 'svelte/store';
	import { graph } from '$lib/wailsjs/go/models';
	import ConfirmDeleteWorkspaceModal from './ConfirmDeleteWorkspaceModal.svelte';

	let name = $state('');
	let saving = $state(false);

	$effect(() => {
		const g = get(workspaceGraphStore);
		if (g?.name != null) name = g.name;
	});

	async function save() {
		const g = get(workspaceGraphStore);
		if (!g?.id || !name.trim()) return;

		saving = true;
		const [, err] = await tryCatch(UpdateName, g.id, name.trim());
		saving = false;
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to update name' });
			return;
		}

		notify({ type: AlertType.Success, message: 'Workspace name updated' });

		workspaceGraphStore.set({ ...g, name: name.trim() } as graph.WorkspaceNode);
	}

	function openDeleteConfirm() {
		const g = get(workspaceGraphStore);
		if (!g?.id) return;
		modalStore.set({
			content: (() => ConfirmDeleteWorkspaceModal) as () => Component,
			props: {
				workspaceName: g.name || 'This workspace',
				onClose: () => modalStore.set(null),
				onConfirm: async () => {
					const [, err] = await tryCatch(DeleteWorkspace, g.id);
					if (err) {
						notify({
							type: AlertType.Error,
							message: err?.message ?? 'Failed to delete workspace'
						});
						modalStore.set(null);
						return;
					}

					modalStore.set(null);
					await Logout();
				}
			},
			width: 400
		});
	}
</script>

<div class="workspace-panel">
	<div class="section space x y">
		<p class="section-title">Workspace name</p>
		<div class="field">
			<Input bind:value={name} placeholder="My Workspace" />
		</div>
		<div class="actions">
			<Button content="Save" emphasis="high" size="sm" onclick={save} disabled={saving} />
		</div>
	</div>
	<div class="section space x y danger">
		<p class="section-title">Danger zone</p>
		<div class="delete-btn">
			<Button content="Delete workspace" emphasis="warning" onclick={openDeleteConfirm} />
		</div>
	</div>
</div>

<style>
	.workspace-panel {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm-md);
		max-width: 480px;
		height: 100%;
	}
	.section {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}
	.space.x.y {
		padding: var(--space-sm-md);
	}
	.section-title {
		font-size: var(--fs-xs);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--gray-800);
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}
	.actions {
		margin-top: var(--space-xs);
		display: flex;
		gap: var(--space-xs);
	}
	.danger {
		margin: var(--space-sm-md) var(--space-sm-md) var(--space-sm-md) var(--space-sm-md);
		border: var(--border);
		background-color: var(--gray-400);
		border-radius: var(--br-xs);
		margin-top: auto;
	}
	:global([data-theme='light']) .delete-btn :global(.button.high p) {
		color: var(--gray-100);
	}
</style>
