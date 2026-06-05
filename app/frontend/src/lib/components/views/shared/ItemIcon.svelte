<script lang="ts">
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { expandedItemIdsStore } from '$lib/components/views/shared/sharedStore';
	import type { graph } from '$lib/wailsjs/go/models';
	import { getItemIconSlot } from '$lib/components/views/shared/getIcon';
	import DatabaseIndicator from '$lib/components/shared/DatabaseIndicator/DatabaseIndicator.svelte';
	import { expandableItemTypes } from './expandableItemTypes';

	let {
		item,
		muted,
		noDepth
	}: {
		item: graph.FileNode | graph.FolderNode | graph.DBInstanceNode | graph.DBInstanceItemNode;
		muted?: boolean;
		noDepth?: boolean;
	} = $props();

	const slot = $derived.by(() => {
		const expandable = expandableItemTypes.has(item.type);
		const expanded =
			expandable &&
			$expandedItemIdsStore instanceof Map &&
			$expandedItemIdsStore.get(item.id) === true;
		return getItemIconSlot(
			{
				type: item.type,
				name: item.name,
				id: item.id,
				metadata: (item as { metadata?: Record<string, unknown> }).metadata
			},
			{ noDepth: noDepth ?? false, expanded, expandable, muted: muted ?? false }
		);
	});

	const stroke = $derived(muted ? 'var(--gray-700)' : 'var(--gray-800)');
	const fill = $derived(
		slot &&
			(slot.kind === 'folder-only' || slot.kind === 'expandable') &&
			slot.expandIcon === 'folder-open'
			? 'var(--gray-700)'
			: 'none'
	);
</script>

{#if slot?.kind === 'folder-only'}
	<Icon
		icon={slot.expandIcon}
		size={17}
		{stroke}
		{fill}
		class={slot.expandIcon === 'folder-open' ? 'icon-folder-open' : ''}
	/>
{:else if slot?.kind === 'expandable'}
	<Icon
		icon={slot.expandIcon}
		size={17}
		{stroke}
		{fill}
		class={slot.expandIcon === 'folder-open' ? 'icon-folder-open' : ''}
	/>
	{#if slot.content.kind === 'file'}
		<Icon icon={slot.content.icon} size={slot.content.size} stroke={slot.content.color} />
	{:else if slot.content.kind === 'database-indicator'}
		<DatabaseIndicator id={slot.content.id} size={16} loaderSize={14} />
	{:else if slot.content.kind === 'icon'}
		{#if slot.content.spacer}
			<div class="spacer"></div>
		{/if}
		<Icon icon={slot.content.icon} size={16} {stroke} />
	{/if}
{:else if slot?.kind === 'file'}
	<Icon icon={slot.icon} size={slot.size} stroke={slot.color} />
{:else if slot?.kind === 'database-indicator'}
	<DatabaseIndicator id={slot.id} size={16} loaderSize={14} />
{:else if slot?.kind === 'icon'}
	{#if slot.spacer}
		<div class="spacer"></div>
	{/if}
	<Icon icon={slot.icon} size={slot.size ?? 16} {stroke} />
{/if}

<style>
	.spacer {
		width: 4px;
	}
</style>
