<script lang="ts">
	// A workspace is a folder, so switching means picking one. With none open
	// there is nothing to name, and the button says what it does instead.
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import Avatar from '$lib/system/Avatar/Avatar.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Tooltip from '$lib/system/Tooltip/Tooltip.svelte';
	import { logoSrc } from '$lib/utils/workspaceLogo';
	import {
		folderStore,
		openingStore,
		pickAndOpenFolder
	} from '$lib/components/PageFolder/folderStore';
	import { osStore } from '$lib/utils/platform';

	const workspace = $derived($workspaceGraphStore);
	const folderPath = $derived($folderStore?.path ?? '');

	// The chord the keymap binds to workbench.openFolder, spelled for this platform.
	const shortcut = $derived($osStore === 'macos' ? 'Cmd+O' : 'Ctrl+O');
</script>

<div class="slot" class:empty={!workspace} data-test="workspace.button">
	{#if workspace}
		<Tooltip text={folderPath} position="bottom" capitalize={false}>
			<button type="button" class="workspace" onclick={pickAndOpenFolder}>
				<Avatar src={logoSrc(workspace.logo)} name={workspace.name} size={20} shape="rounded" />
				<p class="workspace-name">{workspace.name}</p>
			</button>
		</Tooltip>
	{:else}
		<Button
			content="Open folder"
			leftIcon="folder-open"
			iconSize={18}
			emphasis="high"
			label={`Open a folder (${shortcut})`}
			noCapitalizeTooltip
			loading={$openingStore}
			style="width: 100%; justify-content: start;"
			onclick={pickAndOpenFolder}
		/>
	{/if}
</div>

<style>
	/* The tooltip is what sits in the leftbar row, so the width goes on it. */
	.slot {
		min-width: 0;
		max-width: 100%;
	}

	.slot.empty {
		flex: 1;
	}

	.slot.empty :global(.tooltip-container) {
		display: block;
	}

	/* The system button cannot hold an avatar, so this one is built to match it:
	   same padding, radius, hover and press as a low-emphasis Button. */
	.workspace {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		min-width: 0;
		max-width: 100%;
		min-height: 26px;

		padding: var(--space-xs) var(--space-sm);
		border: none;
		border-radius: var(--br-xs);
		background-color: transparent;
		font: inherit;
		transition:
			background-color 0.1s ease-out 0.03s,
			transform 0.05s ease;
	}

	.workspace:hover {
		background-color: var(--gray-400);
	}

	.workspace:active {
		transform: scale(0.98);
	}

	.workspace-name {
		margin: 0;
		padding: var(--space-xs);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--gray-800);
		transition: color 0.1s ease-out 0.05s;
	}

	.workspace:hover .workspace-name {
		color: var(--gray-1000);
	}
</style>
