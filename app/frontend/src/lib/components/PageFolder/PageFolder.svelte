<script lang="ts">
	// What the workbench shows when no folder is open, in place of the tabs a
	// workspace would have. One shell, since only the reason differs.
	import { fly } from 'svelte/transition';
	import Button from '$lib/system/Button/Button.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Wordmark from '$lib/components/shared/Wordmark/Wordmark.svelte';
	import { Logout } from '$lib/bindings/selectDb/internal/system/system';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { CreateWorkspaceInFolder } from '$lib/bindings/selectDb/internal/workspace/workspace';
	import {
		folderStore,
		foldersStore,
		openFolder,
		openingStore,
		displayFolder
	} from './folderStore';
	import { WorkspaceStatus } from '$lib/bindings/selectDb/internal/workspace/models';

	const folder = $derived($folderStore);

	let workspaceName = $state('');

	// Replacing the config of a workspace the user cannot open is behind a click,
	// so it cannot be the reflex when the real answer is to ask for an invite.
	let replacing = $state(false);

	// The folder's own name, which is what a workspace is usually called. Filled
	// once per folder, so it never overwrites what is being typed.
	let namedFolder = '';
	$effect(() => {
		const current = $folderStore;
		if (!current?.path || current.path === namedFolder) return;
		namedFolder = current.path;
		workspaceName = current.suggestedName ?? '';
	});

	const enter = { y: 6, duration: 260 };

	async function createWorkspace() {
		const current = $folderStore;
		if (!current || !current.path) return;

		const [result, err] = await tryCatch(
			CreateWorkspaceInFolder,
			current.path,
			workspaceName.trim()
		);
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Could not create the workspace' });
			return;
		}
		await displayFolder(result);
	}
</script>

{#snippet nameField()}
	<div class="field">
		<p class="label">Workspace name</p>
		<Input bind:value={workspaceName} placeholder="Workspace name" autofocus />
	</div>
{/snippet}

<div class="wrapper">
	<div class="panel" data-test="folder.screen" data-test-value={folder?.status ?? ''}>
		{#if !folder}
			<Loader size={24} />
		{:else if folder.status === WorkspaceStatus.WrongServer}
			<div class="logo-row">
				<Wordmark height="1.1rem" color="var(--gray-800)" />
			</div>
			<div class="divider"></div>
			<div class="state" in:fly={enter}>
				<h1>This folder belongs to another server</h1>
				<p class="path">{folder.path}</p>
				<p class="hint">
					It is a workspace on <strong>{folder.folderServer}</strong>, and you are signed in to
					<strong>{folder.currentServer}</strong>. Sign in there to open it.
				</p>
				<div class="actions">
					<Button content="Sign out" emphasis="high" onclick={() => Logout()} />
				</div>
			</div>
		{:else if folder.status === WorkspaceStatus.NoAccess}
			<div class="logo-row">
				<Wordmark height="1.1rem" color="var(--gray-800)" />
			</div>
			<div class="divider"></div>
			<div class="state" in:fly={enter}>
				<h1>You cannot open this workspace</h1>
				<p class="path">{folder.path}</p>
				<p class="hint">
					It was deleted, or your access to it was removed. Ask someone in the workspace to invite
					you.
				</p>
				<div class="actions">
					<Button
						content="Try again"
						emphasis="high"
						loading={$openingStore}
						onclick={() => openFolder(folder.path)}
					/>
				</div>

				<div class="aside">
					{#if replacing}
						<p class="hint" in:fly={enter}>
							This overwrites <code>select.config.json</code>, which your team may share.
						</p>
						{@render nameField()}
						<div class="actions">
							<Button content="Create workspace" emphasis="warning" onclick={createWorkspace} />
						</div>
					{:else}
						<button class="link" onclick={() => (replacing = true)}>
							Start a new workspace in this folder
						</button>
					{/if}
				</div>
			</div>
		{:else if folder.status === WorkspaceStatus.NeedsSetup}
			<div class="state" in:fly={enter}>
				<h1>Set up this folder</h1>
				<p class="path">{folder.path}</p>

				{@render nameField()}

				<div class="actions">
					<Button content="Create workspace" emphasis="high" onclick={createWorkspace} />
				</div>

				<p class="hint">
					This adds a <code>select.config.json</code> naming the workspace. Nothing else in the folder
					changes.
				</p>
			</div>
		{:else if folder.status === WorkspaceStatus.NoFolder}
			<div class="logo-row">
				<Wordmark height="1.1rem" color="var(--gray-800)" />
			</div>
			<div class="divider"></div>
			<div class="state" in:fly={enter}>
				<p class="eyebrow">Recent</p>
				{#if $foldersStore.length}
					<ul class="recent-list">
						{#each $foldersStore as recent (recent.path)}
							<li>
								<button
									class="recent-card"
									aria-label={`Reopen ${recent.name}`}
									onclick={() => openFolder(recent.path)}
								>
									<Icon icon="folder" size={16} stroke="var(--gray-700)" />
									<span class="recent-text">
										<span class="recent-name">{recent.name}</span>
										<span class="recent-path">{recent.path}</span>
									</span>
									<Icon icon="chevron-right" size={15} stroke="var(--gray-700)" />
								</button>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="hint">No folder opened yet.</p>
				{/if}
			</div>
		{:else}
			<!-- ready: the tree is still loading -->
			<Loader size={24} />
		{/if}
	</div>
</div>

<style>
	/* The workbench with no tabs open: everything starts at the top left, under
	   the wordmark, like the empty tab area it stands in for. */
	.wrapper {
		display: flex;
		flex-direction: column;
		align-items: start;
		gap: var(--space-sm);
		width: 100%;
		height: 100%;
		padding: var(--space-md) 0;
		box-sizing: border-box;
		overflow: auto;
	}

	.logo-row {
		padding-left: var(--space-md);
		padding-bottom: var(--space-xs);
	}

	.divider {
		width: 100%;
		border-bottom: var(--border);
	}

	.panel {
		display: flex;
		flex-direction: column;
		width: 100%;
		max-width: 460px;
		padding: var(--space-sm) var(--space-md);
		box-sizing: border-box;
	}

	.state {
		display: flex;
		flex-direction: column;
		align-items: start;
		gap: var(--space-md);
		width: 100%;
	}

	h1 {
		margin: 0;
		font-size: var(--fs-lg);
		font-weight: var(--fw-bold);
		letter-spacing: -0.01em;
		color: var(--gray-1000);
	}

	.hint {
		margin: 0;
		font-size: var(--fs-sm);
		line-height: 1.6;
		text-wrap: pretty;
		color: var(--gray-800);
	}

	.path {
		max-width: 100%;
		margin: 0;
		padding: var(--space-xs) var(--space-sm);
		border: var(--border);
		border-radius: var(--br-sm);
		background-color: var(--gray-200);
		font-family: 'JetBrains Mono', monospace;
		font-size: var(--fs-xs);
		word-break: break-all;
		color: var(--gray-800);
	}

	code {
		font-family: 'JetBrains Mono', monospace;
		font-size: var(--fs-xs);
		color: var(--gray-900);
		word-break: break-all;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		width: 100%;
	}

	.label {
		margin: 0;
		font-size: var(--fs-xs);
		color: var(--gray-700);
	}

	.eyebrow {
		margin: 0;
		font-size: var(--fs-xxs);
		font-weight: var(--fw-md);
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--gray-700);
	}

	.recent-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		width: 100%;
		list-style: none;
	}

	.recent-card {
		display: flex;
		align-items: center;
		gap: var(--space-sm-md);
		width: 100%;
		padding: var(--space-sm) var(--space-sm-md);
		border: var(--border);
		border-radius: var(--br-sm);
		background-color: var(--gray-200);
		box-shadow: var(--shadow-subtle);
		transition:
			background-color 0.1s ease-out 0.03s,
			border-color 0.1s ease-out 0.03s;
	}

	.recent-card:hover {
		background-color: var(--gray-300);
		border-color: var(--border-color-contrast);
	}

	.recent-text {
		display: flex;
		flex-direction: column;
		gap: var(--space-xxs);
		flex: 1;
		min-width: 0;
		text-align: left;
	}

	.recent-name {
		font-size: var(--fs-sm);
		font-weight: var(--fw-md);
		color: var(--gray-1000);
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.recent-path {
		font-family: 'JetBrains Mono', monospace;
		font-size: var(--fs-xxs);
		color: var(--gray-700);
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* The way out of a dead end, not the answer to it: separated from the button
	   that is. */
	.aside {
		display: flex;
		flex-direction: column;
		align-items: start;
		gap: var(--space-md);
		width: 100%;
		padding-top: var(--space-md);
		border-top: var(--border);
	}

	.link {
		padding: 0;
		border: none;
		background: none;
		font-size: var(--fs-xs);
		color: var(--gray-700);
		text-decoration: underline;
		text-underline-offset: 2px;
		transition: color 0.1s ease-out 0.05s;
	}

	.link:hover {
		color: var(--gray-1000);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-sm);
	}
</style>
