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
		lastFolderStore,
		openFolder,
		openingStore,
		pickAndOpenFolder,
		displayFolder
	} from './folderStore';
	import { WorkspaceStatus } from '$lib/bindings/selectDb/internal/workspace/models';

	const folder = $derived($folderStore);

	// Empty means "use the folder's name", which the backend fills in.
	let workspaceName = $state('');

	// Replacing the config of a workspace the user cannot open is behind a click,
	// so it cannot be the reflex when the real answer is to ask for an invite.
	let replacing = $state(false);

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

{#snippet pathChip(path: string)}
	<p class="path">{path}</p>
{/snippet}

{#snippet nameField()}
	<div class="field">
		<p class="label">Workspace name</p>
		<Input bind:value={workspaceName} placeholder="Workspace name" autofocus />
	</div>
{/snippet}

<div class="wrapper">
	<div class="panel" data-test="folder.screen" data-test-value={folder?.status ?? ''}>
		<Wordmark height="1.05rem" color="var(--gray-700)" />

		{#if !folder}
			<Loader size={24} />
		{:else if folder.status === WorkspaceStatus.WrongServer}
			<div class="state" in:fly={enter}>
				<span class="pill">
					<Icon icon="server" size={13} stroke="var(--orange)" />
					Different server
				</span>
				<h1>This folder belongs to another server</h1>
				{@render pathChip(folder.path)}
				<p class="hint">
					It is a workspace on <strong>{folder.folderServer}</strong>, and you are signed in to
					<strong>{folder.currentServer}</strong>.
				</p>
				<p class="hint">
					Roles and permissions come from the server a workspace lives on, so this folder cannot be
					opened until you sign in there. Sign out, pick
					<strong>{folder.folderServer}</strong> on the sign-in screen, and open the folder again.
				</p>
				<div class="actions">
					<Button content="Sign out" emphasis="high" onclick={() => Logout()} />
					<Button
						content="Open another folder"
						loading={$openingStore}
						onclick={pickAndOpenFolder}
					/>
				</div>
			</div>
		{:else if folder.status === WorkspaceStatus.NoAccess}
			<div class="state" in:fly={enter}>
				<span class="pill">
					<Icon icon="key" size={13} stroke="var(--orange)" />
					No access
				</span>
				<h1>You cannot open this workspace</h1>
				{@render pathChip(folder.path)}
				<p class="hint">
					It belongs to a workspace on <strong>{folder.currentServer}</strong> that is not yours to open:
					it was deleted, or your access to it was removed.
				</p>
				<p class="hint">
					Ask someone in the workspace to invite you, then open the folder again. Your files are
					untouched either way.
				</p>

				<div class="actions">
					<Button
						content="Open another folder"
						emphasis="high"
						loading={$openingStore}
						onclick={pickAndOpenFolder}
					/>
					<Button
						content="Try again"
						loading={$openingStore}
						onclick={() => openFolder(folder.path)}
					/>
				</div>

				<div class="aside">
					{#if replacing}
						<p class="hint" in:fly={enter}>
							This overwrites <code>select.config.json</code>, which your team may share. Everything
							else in the folder stays as it is.
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
				<span class="pill">
					<Icon icon="plus" size={13} stroke="var(--green)" />
					New workspace
				</span>
				<h1>Set up this folder</h1>
				{@render pathChip(folder.path)}

				{@render nameField()}

				<div class="actions">
					<Button content="Create workspace" emphasis="high" onclick={createWorkspace} />
					<Button
						content="Open another folder"
						loading={$openingStore}
						onclick={pickAndOpenFolder}
					/>
				</div>

				<p class="hint footnote">
					This adds a <code>select.config.json</code> naming the workspace, and nothing else. Your files
					stay exactly as they are, and an empty folder also gets a small sample database to start from.
				</p>
			</div>
		{:else if folder.status === WorkspaceStatus.NoFolder}
			<div class="state" in:fly={enter}>
				<h1 class="hero">Open a folder to start</h1>
				<p class="hint lede">
					A workspace is a folder on your machine. Open one and SELECT reads what is already in it.
				</p>

				<ul class="traits">
					<li><Icon icon="sql" size={14} stroke="var(--gray-700)" /> SQL files</li>
					<li><Icon icon="db" size={14} stroke="var(--gray-700)" /> Database configs</li>
					<li><Icon icon="eslint" size={14} stroke="var(--gray-700)" /> Lint rules</li>
				</ul>

				<div class="cta">
					<Button
						content="Open folder"
						leftIcon="folder-open"
						iconSize={18}
						emphasis="high"
						loading={$openingStore}
						onclick={pickAndOpenFolder}
					/>
				</div>

				{#if $lastFolderStore}
					<div class="recent">
						<p class="eyebrow">Recent</p>
						<button
							class="recent-card"
							aria-label={`Reopen ${$lastFolderStore.name}`}
							onclick={() => openFolder($lastFolderStore!.path)}
						>
							<Icon icon="folder" size={16} stroke="var(--gray-700)" />
							<span class="recent-text">
								<span class="recent-name">{$lastFolderStore.name}</span>
								<span class="recent-path">{$lastFolderStore.path}</span>
							</span>
							<Icon icon="chevron-right" size={15} stroke="var(--gray-700)" />
						</button>
					</div>
				{/if}
			</div>
		{:else}
			<!-- ready: the tree is still loading -->
			<Loader size={24} />
		{/if}
	</div>
</div>

<style>
	.wrapper {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 100%;
		height: 100%;
		overflow: auto;
	}

	/* Lifts the panel off a flat backdrop. Too faint to read as a colour, which
	   is the point: it has to survive both themes. */
	.wrapper::after {
		content: '';
		position: absolute;
		inset: 0;
		pointer-events: none;
		background: radial-gradient(
			58% 42% at 50% 36%,
			color-mix(in srgb, var(--blue-glow) 8%, transparent),
			transparent 70%
		);
	}

	.panel {
		position: relative;
		z-index: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-lg);
		width: 100%;
		max-width: 440px;
		padding: var(--space-lg);
		text-align: center;
		box-sizing: border-box;
	}

	.state {
		display: flex;
		flex-direction: column;
		align-items: center;
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

	h1.hero {
		font-size: var(--fs-xxl);
		letter-spacing: -0.02em;
	}

	.hint {
		margin: 0;
		font-size: var(--fs-sm);
		line-height: 1.6;
		text-wrap: pretty;
		white-space: normal;
		color: var(--gray-800);
	}

	.lede {
		max-width: 34ch;
	}

	.footnote {
		font-size: var(--fs-xs);
		color: var(--gray-700);
	}

	/* Names the state before the headline does, so the shape of the screen is
	   readable before a word of it is. */
	.pill {
		display: inline-flex;
		align-items: center;
		gap: var(--space-xs);
		padding: var(--space-xxs) var(--space-sm);
		border: var(--border);
		border-radius: var(--br-xl);
		background-color: var(--gray-200);
		font-size: var(--fs-xs);
		font-weight: var(--fw-md);
		letter-spacing: 0.04em;
		text-transform: uppercase;
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
		white-space: normal;
		word-break: break-all;
		color: var(--gray-800);
	}

	code {
		font-family: 'JetBrains Mono', monospace;
		font-size: var(--fs-xs);
		color: var(--gray-900);
		word-break: break-all;
	}

	.traits {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: var(--space-xs) var(--space-md);
		list-style: none;
	}

	.traits li {
		display: inline-flex;
		align-items: center;
		gap: var(--space-xs);
		font-size: var(--fs-xs);
		color: var(--gray-700);
	}

	.cta :global(.button) {
		padding: var(--space-xs-sm) var(--space-md) var(--space-xs-sm) var(--space-sm-md);
	}

	.cta :global(.button p) {
		font-size: var(--fs-md);
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		width: 100%;
		text-align: left;
	}

	.label {
		margin: 0;
		font-size: var(--fs-xs);
		color: var(--gray-700);
	}

	.recent {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		width: 100%;
		padding-top: var(--space-md);
		border-top: var(--border);
	}

	.eyebrow {
		margin: 0;
		font-size: var(--fs-xxs);
		font-weight: var(--fw-md);
		letter-spacing: 0.08em;
		text-transform: uppercase;
		text-align: left;
		color: var(--gray-700);
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
		cursor: pointer;
		transition:
			background-color 0.12s ease-out,
			border-color 0.12s ease-out;
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

	/* The way out of a dead end, not the answer to it: separated from the two
	   buttons that are. */
	.aside {
		display: flex;
		flex-direction: column;
		align-items: center;
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
		cursor: pointer;
	}

	.link:hover {
		color: var(--gray-1000);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: var(--space-sm);
	}
</style>
