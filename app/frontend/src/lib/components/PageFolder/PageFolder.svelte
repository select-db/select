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
	import { InitWorkspaceInFolder } from '$lib/bindings/selectDb/internal/workspace/workspace';
	import {
		folderStore,
		lastFolderStore,
		openFolder,
		pickAndOpenFolder,
		refreshLastFolder
	} from './folderStore';

	const view = $derived($folderStore);

	let workspaceName = $state('');
	let initialisedFor = $state('');

	// The folder's own name is what the user would have typed, so it is what the
	// field starts with. Reset when the folder changes, not on every render.
	$effect(() => {
		const s = $folderStore;
		if (s?.state === 'needs_init' && initialisedFor !== s.path) {
			initialisedFor = s.path;
			workspaceName = s.suggestedName ?? '';
		}
	});

	$effect(() => {
		if ($folderStore?.state === 'no_folder') void refreshLastFolder();
	});

	async function createWorkspace() {
		const s = $folderStore;
		if (s?.state !== 'needs_init') return;

		const [result, err] = await tryCatch(InitWorkspaceInFolder, s.path, workspaceName.trim());
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Could not create the workspace' });
			return;
		}
		folderStore.set(result);
		// The graph load lives in the store's open path; reopening by path is the
		// shortest way to reuse it rather than repeat it here.
		await openFolder(s.path);
	}
</script>

<div class="wrapper">
	<div class="panel">
		{#if !view}
			<Loader size={24} />
		{:else if view.state === 'wrong_server'}
			<h1>This folder belongs to another server</h1>
			<p class="hint">
				<code>{view.path}</code> is a workspace on
				<strong>{view.folderServer}</strong>, and you are signed in to
				<strong>{view.currentServer}</strong>.
			</p>
			<p class="hint">
				Roles and permissions come from the server a workspace lives on, so this folder cannot
				be opened until you sign in there. Sign out, pick
				<strong>{view.folderServer}</strong> on the sign-in screen, and open the folder again.
			</p>
			<div class="actions">
				<Button content="Sign out" emphasis="high" onclick={() => Logout()} />
				<Button content="Open another folder" onclick={pickAndOpenFolder} />
			</div>
		{:else if view.state === 'needs_init'}
			<h1>{view.staleWorkspaceId ? 'This workspace no longer exists' : 'Set up this folder'}</h1>
			<p class="hint">
				<code>{view.path}</code>
			</p>
			{#if view.staleWorkspaceId}
				<p class="hint">
					The folder says it belongs to a workspace this server does not have, or that you are
					not a member of. Creating a workspace here replaces that reference; nothing else in
					the folder is touched.
				</p>
			{:else}
				<p class="hint">
					Creating a workspace here adds a <code>select.config.json</code> naming it, and
					nothing else. Your files stay exactly as they are. An empty folder also gets a small
					sample database to start from.
				</p>
			{/if}

			<div class="field">
				<p class="label">Workspace name</p>
				<Input bind:value={workspaceName} placeholder="Workspace name" autofocus />
			</div>

			<div class="actions">
				<Button
					content="Create workspace"
					emphasis="high"
					disabled={!workspaceName.trim()}
					onclick={createWorkspace}
				/>
				<Button content="Open another folder" onclick={pickAndOpenFolder} />
			</div>
		{:else}
			<h1>No folder open</h1>
			<p class="hint">
				A workspace is a folder on your machine. Open one to start, and SELECT reads the SQL
				files, database configs and lint rules already in it.
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
		{/if}
	</div>
</div>

<style>
	.wrapper {
		/* The window is frameless and this screen renders no tab bar, so the
		   backdrop is the drag handle, as on the sign-in screen. */
		--wails-draggable: drag;

		display: flex;
		align-items: center;
		justify-content: center;
		width: 100vw;
		height: 100vh;
		background-color: var(--gray-0);
	}

	/* --wails-draggable inherits, so anything interactive has to opt back out or
	   its mousedown starts a window drag instead of reaching the control. */
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
