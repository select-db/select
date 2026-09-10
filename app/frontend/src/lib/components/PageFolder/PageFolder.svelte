<script lang="ts">
	// The screens between signing in and looking at files. One shell, since only
	// the reason for not having a folder open differs.
	import Button from '$lib/system/Button/Button.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { Logout } from '$lib/bindings/selectDb/internal/system/system';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { CreateWorkspaceInFolder } from '$lib/bindings/selectDb/internal/workspace/workspace';
	import {
		folderStore,
		lastFolderStore,
		openFolder,
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

<div class="wrapper">
	<div class="panel" data-test="folder.screen" data-test-value={folder?.status ?? ''}>
		{#if !folder}
			<Loader size={24} />
		{:else if folder.status === WorkspaceStatus.WrongServer}
			<h1>This folder belongs to another server</h1>
			<p class="hint">
				<code>{folder.path}</code> is a workspace on
				<strong>{folder.folderServer}</strong>, and you are signed in to
				<strong>{folder.currentServer}</strong>.
			</p>
			<p class="hint">
				Roles and permissions come from the server a workspace lives on, so this folder cannot be
				opened until you sign in there. Sign out, pick
				<strong>{folder.folderServer}</strong> on the sign-in screen, and open the folder again.
			</p>
			<div class="actions">
				<Button content="Sign out" emphasis="high" onclick={() => Logout()} />
				<Button content="Open another folder" onclick={pickAndOpenFolder} />
			</div>
		{:else if folder.status === WorkspaceStatus.NoAccess}
			<h1>You cannot open this workspace</h1>
			<p class="hint">
				<code>{folder.path}</code> belongs to a workspace on
				<strong>{folder.currentServer}</strong> that is not yours to open: it was deleted, or your access
				to it was removed.
			</p>
			<p class="hint">
				Ask someone in the workspace to invite you, then open the folder again. Your files are
				untouched either way.
			</p>

			<div class="actions">
				<Button content="Open another folder" emphasis="high" onclick={pickAndOpenFolder} />
				<Button content="Try again" onclick={() => openFolder(folder.path)} />
			</div>

			<button class="link" onclick={() => (replacing = true)}>
				Start a new workspace in this folder
			</button>
			{#if replacing}
				<p class="hint">
					This overwrites <code>select.config.json</code>, which your team may share. Everything
					else in the folder stays as it is.
				</p>
				<div class="field">
					<p class="label">Workspace name</p>
					<Input bind:value={workspaceName} placeholder="Workspace name" autofocus />
				</div>
				<div class="actions">
					<Button content="Create workspace" emphasis="warning" onclick={createWorkspace} />
				</div>
			{/if}
		{:else if folder.status === WorkspaceStatus.NeedsSetup}
			<h1>Set up this folder</h1>
			<p class="hint">
				<code>{folder.path}</code>
			</p>
			<p class="hint">
				Creating a workspace here adds a <code>select.config.json</code> naming it, and nothing else.
				Your files stay exactly as they are. An empty folder also gets a small sample database to start
				from.
			</p>

			<div class="field">
				<p class="label">Workspace name</p>
				<Input bind:value={workspaceName} placeholder="Workspace name" autofocus />
			</div>

			<div class="actions">
				<Button content="Create workspace" emphasis="high" onclick={createWorkspace} />
				<Button content="Open another folder" onclick={pickAndOpenFolder} />
			</div>
		{:else if folder.status === WorkspaceStatus.NoFolder}
			<h1>No folder open</h1>
			<p class="hint">
				A workspace is a folder on your machine. Open one to start, and SELECT reads the SQL files,
				database configs and lint rules already in it.
			</p>

			<div class="actions">
				<Button
					content="Open folder"
					leftIcon="folder-open"
					emphasis="high"
					onclick={pickAndOpenFolder}
				/>
				{#if $lastFolderStore}
					<Button
						content={`Reopen ${$lastFolderStore.name}`}
						onclick={() => openFolder($lastFolderStore!.path)}
					/>
				{/if}
			</div>

			{#if $lastFolderStore}
				<p class="hint path">{$lastFolderStore.path}</p>
			{/if}
		{:else}
			<!-- ready: the tree is still loading -->
			<Loader size={24} />
		{/if}
	</div>
</div>

<style>
	.wrapper {
		/* Frameless window, no tab bar: the backdrop is the drag handle. */
		--wails-draggable: drag;

		display: flex;
		align-items: center;
		justify-content: center;
		width: 100vw;
		height: 100vh;
		background-color: var(--gray-0);
	}

	/* --wails-draggable inherits, so controls have to opt back out or their
	   mousedown starts a window drag. */
	.actions,
	.field,
	.wrapper :global(.link),
	.wrapper :global(button),
	.wrapper :global(input) {
		--wails-draggable: no-drag;
	}

	.panel {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-md);
		text-align: center;
		max-width: 460px;
		padding: var(--space-lg);
		box-sizing: border-box;
	}

	h1 {
		margin: 0;
		font-size: var(--font-size-lg, 1.1rem);
		font-weight: 600;
		color: var(--gray-900);
	}

	.hint {
		margin: 0;
		font-size: var(--font-size-sm, 0.85rem);
		line-height: 1.5;
		color: var(--gray-600);
	}

	.hint.path {
		word-break: break-all;
		opacity: 0.75;
	}

	code {
		font-family: var(--font-mono, monospace);
		word-break: break-all;
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
		font-size: var(--font-size-sm, 0.85rem);
		color: var(--gray-600);
	}

	.link {
		padding: 0;
		border: none;
		background: none;
		font-size: var(--font-size-sm, 0.85rem);
		color: var(--gray-600);
		text-decoration: underline;
		cursor: pointer;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: var(--space-sm);
		margin-top: var(--space-sm);
	}
</style>
