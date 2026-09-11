<script lang="ts">
	// A workspace is a folder, so switching means picking one. With none open
	// there is nothing to name, and the button says what it does instead.
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import Avatar from '$lib/system/Avatar/Avatar.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Tooltip from '$lib/system/Tooltip/Tooltip.svelte';
	import { logoSrc } from '$lib/utils/workspaceLogo';
	import { folderStore, pickAndOpenFolder } from '$lib/components/PageFolder/folderStore';

	const workspace = $derived($workspaceGraphStore);
	const folderPath = $derived($folderStore?.path ?? '');
</script>

<Tooltip
	text={workspace ? folderPath : 'Open a folder on your machine'}
	position="bottom"
	capitalize={false}
>
	<button
		type="button"
		class="workspace-button"
		class:empty={!workspace}
		data-test="workspace.button"
		onclick={pickAndOpenFolder}
	>
		{#if workspace}
			<Avatar src={logoSrc(workspace.logo)} name={workspace.name} size={20} shape="rounded" />
			<span class="workspace-name">{workspace.name}</span>
		{:else}
			<Icon icon="folder-open" size={16} stroke="var(--gray-800)" />
			<span class="workspace-name">Open folder</span>
		{/if}
	</button>
</Tooltip>

<style>
	.workspace-button {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		min-width: 0;
		max-width: 100%;

		padding: var(--space-xs) var(--space-sm);
		border: none;
		border-radius: var(--br-xs);
		background: transparent;
		color: inherit;
		font: inherit;
		cursor: pointer;
	}

	.workspace-button:hover {
		background-color: var(--gray-100);
	}

	.workspace-button.empty {
		gap: var(--space-xs);
		border: var(--border);
		background-color: var(--gray-200);
		box-shadow: var(--shadow-subtle);
	}

	.workspace-button.empty:hover {
		background-color: var(--gray-300);
	}

	.workspace-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
