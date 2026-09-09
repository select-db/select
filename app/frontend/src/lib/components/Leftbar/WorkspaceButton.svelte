<script lang="ts">
	// A workspace is a folder, so switching means picking one.
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import Avatar from '$lib/system/Avatar/Avatar.svelte';
	import Tooltip from '$lib/system/Tooltip/Tooltip.svelte';
	import { logoSrc } from '$lib/utils/workspaceLogo';
	import { folderStore, pickAndOpenFolder } from '$lib/components/PageFolder/folderStore';

	const workspace = $derived($workspaceGraphStore);
	const folderPath = $derived($folderStore?.path ?? '');
</script>

<Tooltip text={folderPath || 'Open a folder'} position="bottom" capitalize={false}>
	<button type="button" class="workspace-button" onclick={pickAndOpenFolder}>
		<Avatar src={logoSrc(workspace?.logo)} name={workspace?.name} size={20} shape="rounded" />
		<span class="workspace-name">{workspace?.name ?? 'Open folder'}</span>
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
		border-radius: var(--radius-sm, 4px);
		background: transparent;
		color: inherit;
		font: inherit;
		cursor: pointer;
	}

	.workspace-button:hover {
		background-color: var(--gray-100);
	}

	.workspace-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
