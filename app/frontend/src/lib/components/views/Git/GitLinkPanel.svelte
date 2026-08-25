<script lang="ts">
	import { type Component } from 'svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { AlertType } from '$lib/system/Alert/types';
	import { gitWorkspaceStatusStore, loadGitStatus } from '$lib/components/views/Git/gitStore';
	import LinkOptionsModal from '$lib/components/views/Git/LinkOptionsModal.svelte';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import {
		CompleteLinkExistingRepo,
		LinkExistingRepo,
		UnlinkRemote
	} from '$lib/bindings/selectDb/internal/git/git';

	let { showUnsync = false }: { showUnsync?: boolean } = $props();

	const workspaceGitStatus = $derived($gitWorkspaceStatusStore);
	let remoteUrl = $state('');

	const linkRemote = async () => {
		const status = await must(tryCatch(LinkExistingRepo, { remoteUrl }));
		if (status.scenario === 'checkout') {
			modalStore.set({
				content: (() => LinkOptionsModal) as () => Component,
				width: 400,
				props: {
					branch: status.branch,
					onChoice: async (choice: 'checkout' | 'keep') => {
						await must(tryCatch(CompleteLinkExistingRepo, choice));
						notify({ type: AlertType.Success, message: 'Remote linked' });
						await loadGitStatus();
					}
				}
			});
			return;
		}
		notify({ type: AlertType.Success, message: 'Remote linked' });
		await loadGitStatus();
	};

	const unsync = async () => {
		await must(tryCatch(UnlinkRemote));
		notify({ type: AlertType.Success, message: 'Workspace unlinked from remote' });
		await loadGitStatus();
	};
</script>

<div class="git-link-panel">
	{#if !workspaceGitStatus}
		<div class="section space x y">
			<Loader />
		</div>
	{:else if !workspaceGitStatus?.gitAvailable}
		<div class="section space x y">
			<p class="section-title">Git not available</p>
			<p class="hint">
				Git is not installed or not available in your PATH. <br />
				Install Git to enable linking a repository.
			</p>
		</div>
	{:else if workspaceGitStatus.isGitRepo && workspaceGitStatus.hasRemote}
		<div class="section space x y">
			<p class="section-title">Repository</p>
			<p class="hint">This workspace is linked to a remote.</p>
			{#if workspaceGitStatus.remoteUrl}
				<p class="hint"><code>{workspaceGitStatus.remoteUrl}</code></p>
			{/if}
			{#if showUnsync}
				<div class="actions">
					<Button content="Unsync" size="sm" emphasis="high" onclick={unsync} />
				</div>
			{/if}
		</div>
	{:else}
		<div class="section space x y">
			<div class="status-header">
				<p class="section-title">Current status</p>
			</div>
			{#if !workspaceGitStatus.isGitRepo}
				<p class="hint">This workspace is not yet a Git repository.</p>
			{:else}
				<p class="hint">Git repository detected with no configured remote.</p>
			{/if}
		</div>
		<div class="section space x">
			<p class="section-title">Link repository</p>
			<div class="field">
				<p class="label">Remote URL</p>
				<Input
					bind:value={remoteUrl}
					placeholder="git@github.com:owner/repo.git or https://github.com/owner/repo.git"
				/>
			</div>
			<div class="actions">
				<Button
					content={workspaceGitStatus.isGitRepo ? 'Update remote' : 'Link repository'}
					emphasis="high"
					onclick={linkRemote}
					size="sm"
					disabled={!remoteUrl}
				/>
			</div>
		</div>
	{/if}
</div>

<style>
	.git-link-panel {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm-md);
		max-width: 480px;
	}

	.section {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}

	.space.x {
		padding-left: var(--space-sm-md);
		padding-right: var(--space-sm-md);
	}
	.space.y {
		padding-top: var(--space-sm-md);
		padding-bottom: var(--space-sm-md);
	}

	.status-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-xs);
	}

	.section-title {
		font-size: var(--fs-xs);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--gray-800);
	}

	.hint {
		font-size: var(--fs-xs);
		color: var(--gray-800);
	}

	.hint code {
		font-size: var(--fs-xs);
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

	.label {
		color: var(--gray-800);
		font-size: var(--fs-xs);
	}
</style>
