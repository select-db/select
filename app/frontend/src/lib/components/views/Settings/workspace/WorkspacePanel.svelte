<script lang="ts">
	import type { Component } from 'svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { UpdateName, DeleteWorkspace } from '$lib/bindings/selectDb/internal/workspace/workspace';
	import {
		Logout,
		UpdateWorkspaceExecutionLimits
	} from '$lib/bindings/selectDb/internal/system/system';
	import { get } from 'svelte/store';
	import * as graph from '$lib/wails/graph';
	import ConfirmDeleteWorkspaceModal from './ConfirmDeleteWorkspaceModal.svelte';

	let name = $state('');
	let saving = $state(false);

	// Execution limits are team policy stored on the workspace (synced via the
	// backend), shared with everyone in the workspace.
	let statementTimeoutMs = $state(30000);
	let maxResultSizeMb = $state(100);

	$effect(() => {
		const g = get(workspaceGraphStore);
		if (g?.name != null) name = g.name;
		if (g?.statement_timeout_ms != null) statementTimeoutMs = g.statement_timeout_ms;
		if (g?.max_result_size_mb != null) maxResultSizeMb = g.max_result_size_mb;
	});

	// Saves the whole workspace settings form (name + execution limits) at once.
	async function save() {
		const g = get(workspaceGraphStore);
		if (!g?.id) return;

		saving = true;
		const trimmedName = name.trim();

		// Name is required: only update it when it changed and is non-empty.
		if (trimmedName && trimmedName !== g.name) {
			const [, err] = await tryCatch(UpdateName, g.id, trimmedName);
			if (err) {
				saving = false;
				notify({ type: AlertType.Error, message: err?.message ?? 'Failed to save workspace' });
				return;
			}
		}

		const [res, err] = await tryCatch(
			UpdateWorkspaceExecutionLimits,
			Math.trunc(statementTimeoutMs),
			Math.trunc(maxResultSizeMb)
		);
		saving = false;
		if (err || !res) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to save workspace' });
			return;
		}

		// Reflect the normalized values the backend actually stored.
		statementTimeoutMs = res.statement_timeout_ms;
		maxResultSizeMb = res.max_result_size_mb;
		notify({ type: AlertType.Success, message: 'Workspace settings saved' });

		workspaceGraphStore.set({
			...g,
			name: trimmedName || g.name,
			statement_timeout_ms: res.statement_timeout_ms,
			max_result_size_mb: res.max_result_size_mb
		} as graph.WorkspaceNode);
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
		<div></div>
		<p class="section-title">Execution limits</p>
		<div class="field" style="max-width: 150px;">
			<span class="field-label">Statement timeout (ms)</span>
			<Input type="number" min={1} bind:value={statementTimeoutMs} placeholder="30000" />
		</div>
		<div class="field" style="max-width: 150px;">
			<span class="field-label">Max result size (MB)</span>
			<Input type="number" min={1} max={250} bind:value={maxResultSizeMb} placeholder="100" />
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
		gap: var(--space-xs);
		max-width: 275px;
	}
	.field-label {
		font-size: var(--fs-xs);
		color: var(--gray-800);
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
