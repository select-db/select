<script lang="ts">
	// The workspace picker. A workspace is a folder, so the list is the folders
	// this machine has, and Open folder is always there to reach one it does not.
	import Select from '$lib/system/Select/Select.svelte';
	import type { SelectOption } from '$lib/system/Select/Select.types';
	import Avatar from '$lib/system/Avatar/Avatar.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { logoSrc } from '$lib/utils/workspaceLogo';
	import {
		folderStore,
		foldersStore,
		openFolder,
		openingStore,
		pickAndOpenFolder,
		refreshFolders
	} from '$lib/components/PageFolder/folderStore';

	// No folder is at this path, so it cannot collide with one.
	const OPEN_FOLDER = 'select:open-folder';

	const workspace = $derived($workspaceGraphStore);
	const currentPath = $derived($folderStore?.path ?? '');

	// Order is the menu's order, so Open folder leads and the folders follow as
	// ListFolders gives them: the open one first.
	const options = $derived<SelectOption[]>([
		{ value: OPEN_FOLDER, label: 'Open folder' },
		...$foldersStore.map((folder) => ({ value: folder.path, label: folder.name }))
	]);

	// Keyed on the names rather than the stores: the graph store is replaced on
	// every watcher event, and re-listing the folders on each one is a round trip
	// per keystroke somebody else is typing.
	const listKey = $derived(`${$folderStore?.path ?? ''}|${$workspaceGraphStore?.name ?? ''}`);
	$effect(() => {
		void listKey;
		refreshFolders();
	});

	async function choose(value: string) {
		if (value === OPEN_FOLDER) {
			await pickAndOpenFolder();
			return;
		}
		if (value === currentPath) return;
		await openFolder(value);
	}
</script>

{#snippet folderOption(option: SelectOption | null)}
	{#if option?.value === OPEN_FOLDER}
		<span class="option">
			<Icon icon="folder-open" size={18} stroke="var(--gray-800)" />
			<span class="option-label">{option.label}</span>
		</span>
	{:else}
		<span class="option" title={option?.value ?? ''}>
			<Avatar
				src={option && option.value === currentPath ? logoSrc(workspace?.logo) : null}
				name={option?.label}
				size={20}
				shape="rounded"
			/>
			<span class="option-label">{option?.label ?? 'Open folder'}</span>
		</span>
	{/if}
{/snippet}

<div class="slot" class:empty={!workspace} data-test="workspace.button">
	<Select
		value={currentPath}
		{options}
		onchange={(v) => choose(v as string)}
		sortOptions={false}
		isLoading={$openingStore}
		placeholder="Open folder"
		menuWidth={300}
		emphasis="low"
		optionDisplay={folderOption}
	/>
</div>

<style>
	.slot {
		min-width: 0;
		max-width: 100%;
	}

	.slot.empty {
		flex: 1;
	}

	.option {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		min-width: 0;
	}

	.option-label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
