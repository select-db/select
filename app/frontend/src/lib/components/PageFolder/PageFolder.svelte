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

	async function createWorkspace() {
		const current = $folderStore;
		if (current?.status !== WorkspaceStatus.NeedsSetup) return;

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
	<div class="panel">
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
		{:else if folder.status === WorkspaceStatus.NeedsSetup}
			<h1>{folder.staleWorkspaceId ? 'This workspace no longer exists' : 'Set up this folder'}</h1>
			<p class="hint">
				<code>{folder.path}</code>
			</p>
			{#if folder.staleWorkspaceId}
				<p class="hint">
					The folder says it belongs to a workspace this server does not have, or that you are not a
					member of. Creating a workspace here replaces that reference; nothing else in the folder
					is touched.
				</p>
			{:else}
				<p class="hint">
					Creating a workspace here adds a <code>select.config.json</code> naming it, and nothing else.
					Your files stay exactly as they are. An empty folder also gets a small sample database to start
					from.
				</p>
			{/if}

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

	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: var(--space-sm);
		margin-top: var(--space-sm);
	}
</style>
