<script lang="ts">
	import type { Component } from 'svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { tryCatch } from '$lib/utils/tryCatch';
	import {
		UpdateName,
		DeleteWorkspace,
		UpdateLogo
	} from '$lib/bindings/selectDb/internal/workspace/workspace';
	import Avatar from '$lib/system/Avatar/Avatar.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { fileToLogoBase64, logoSrc, LOGO_ACCEPT } from '$lib/utils/workspaceLogo';
	import {
		Logout,
		UpdateWorkspaceExecutionLimits
	} from '$lib/bindings/selectDb/internal/system/system';
	import { get } from 'svelte/store';
	import * as graph from '$lib/wails/graph';
	import ConfirmDeleteWorkspaceModal from './ConfirmDeleteWorkspaceModal.svelte';

	let name = $state('');
	let saving = $state(false);

	// The logo is edited like the rest of the form: picking a file only stages it,
	// Save is what sends it. undefined means "unchanged".
	let stagedLogo = $state<string | undefined>(undefined);
	let logoError = $state('');
	let fileInput = $state<HTMLInputElement | null>(null);

	const currentLogo = $derived($workspaceGraphStore?.logo ?? '');
	const previewLogo = $derived(stagedLogo ?? currentLogo);

	async function pickLogo(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		// Reset the input so picking the same file twice still fires a change.
		input.value = '';
		if (!file) return;

		logoError = '';
		const [base64, err] = await tryCatch(fileToLogoBase64, file);
		if (err || !base64) {
			logoError = err?.message ?? 'Could not read that image';
			return;
		}
		stagedLogo = base64;
	}

	// Sends the staged logo, if any. The server validates and re-encodes it, so
	// what lands in the database is never the bytes the picker produced.
	async function saveLogo(workspaceID: string): Promise<boolean> {
		if (stagedLogo === undefined) return true;

		const [, err] = await tryCatch(UpdateLogo, workspaceID, stagedLogo);
		if (err) {
			logoError = err?.message ?? 'Failed to save logo';
			return false;
		}
		stagedLogo = undefined;
		return true;
	}

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

		if (!(await saveLogo(g.id))) {
			saving = false;
			notify({ type: AlertType.Error, message: logoError });
			return;
		}

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

		// Re-read rather than reusing the snapshot taken at the top of save(): the
		// logo upload rebuilds the graph, so `g` no longer has the current logo.
		const latest = get(workspaceGraphStore) ?? g;
		workspaceGraphStore.set({
			...latest,
			name: trimmedName || latest.name,
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
		<div class="identity">
			<div class="logo-picker">
				<Avatar src={logoSrc(previewLogo)} {name} size={56} shape="rounded" />
				<button
					class="logo-edit"
					type="button"
					title={previewLogo ? 'Replace logo' : 'Upload logo'}
					aria-label={previewLogo ? 'Replace workspace logo' : 'Upload workspace logo'}
					onclick={() => fileInput?.click()}
				>
					<Icon icon="edit" size={12} stroke="var(--gray-1000)" />
				</button>
			</div>
			<div class="name-field">
				<p class="section-title">Workspace name</p>
				<div class="field">
					<Input bind:value={name} placeholder="My Workspace" />
				</div>
			</div>
		</div>
		{#if logoError}
			<span class="logo-error">{logoError}</span>
		{/if}
		<input
			bind:this={fileInput}
			class="logo-input"
			type="file"
			accept={LOGO_ACCEPT}
			onchange={pickLogo}
		/>
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
		gap: var(--space-md);
		max-width: 480px;
		height: 100%;
	}
	.section {
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
	}
	.space.x.y {
		padding: var(--space-md);
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
		max-width: 275px;
	}
	.field-label {
		font-size: var(--fs-xs);
		color: var(--gray-800);
	}
	.identity {
		display: flex;
		align-items: flex-end;
		gap: var(--space-md);
	}
	.name-field {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}
	.logo-picker {
		position: relative;
		flex-shrink: 0;
		line-height: 0;
	}
	/* The edit affordance stays out of the way until the logo is hovered — or the
	   button is focused, so it is still reachable from the keyboard. */
	.logo-edit {
		position: absolute;
		top: -6px;
		right: -6px;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 3px;
		border: var(--border);
		border-radius: 50%;
		background-color: var(--gray-300);
		cursor: pointer;
		opacity: 0;
		transition: opacity 0.1s ease-in-out;
	}
	.logo-picker:hover .logo-edit,
	.logo-edit:focus-visible {
		opacity: 1;
	}
	.logo-edit:hover {
		background-color: var(--gray-400);
	}
	.logo-error {
		font-size: var(--fs-xs);
		color: var(--red);
	}
	.logo-input {
		display: none;
	}
	.actions {
		margin-top: var(--space-sm);
		display: flex;
		gap: var(--space-sm);
	}
	.danger {
		margin: var(--space-md);
		border: var(--border);
		background-color: var(--gray-400);
		border-radius: var(--br-xs);
		margin-top: auto;
	}
	:global([data-theme='light']) .delete-btn :global(.button.high p) {
		color: var(--gray-100);
	}
</style>
