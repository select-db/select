<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import Contextable from '$lib/system/ContextMenu/Contextable.svelte';
	import FieldIndicators from './FieldIndicators.svelte';
	import type { Icons } from '$lib/system/Icon/types';
	import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
	import {
		renamingItemIdStore,
		selectedItemsStore,
		dragStateStore
	} from '$lib/components/views/shared/sharedStore';
	import ItemIcon from './ItemIcon.svelte';
	import ItemName from './ItemName.svelte';
	import { expandableItemTypes } from './expandableItemTypes';
	import { visibleIdsStore } from '$lib/components/views/FileSystem/Files/helpers/visibilityStore';
	import ChildVisibilityBadge from '$lib/components/views/FileSystem/Files/ChildVisibilityBadge.svelte';
	import type { graph } from '$lib/wailsjs/go/models';

	type DisplayProps<T> = {
		item: T;
		options: ContextMenuOption[] | (() => ContextMenuOption[]);
		actions?: {
			icon: Icons;
			onClick(item: T): void;
		}[];
		handleClick(item: T, event?: MouseEvent): void;
		depth?: number;
		parentIds?: string[];
		draggable?: boolean;
		onDragStart?(
			item: graph.FileNode | graph.FolderNode | graph.DBInstanceNode,
			event: DragEvent
		): void;
		onDragOver?(item: graph.FolderNode | graph.DBInstanceNode, event: DragEvent): void;
		onDrop?(item: graph.FolderNode | graph.DBInstanceNode, event: DragEvent): void;
		onDragEnd?: () => void;
	};

	type AnyItem =
		| graph.DBInstanceNode
		| graph.DBInstanceItemNode
		| graph.FolderNode
		| graph.FileNode;

	let {
		item,
		options,
		actions,
		handleClick,
		depth = 0,
		parentIds = [],
		draggable = false,
		onDragStart,
		onDragOver,
		onDrop,
		onDragEnd
	}: DisplayProps<AnyItem> = $props();

	// Dim the item if it declares children but has none (empty)
	const muted = $derived(() => {
		if (!expandableItemTypes.has(item.type)) return false;

		if (item.type === 'folder') {
			const folder = item as graph.FolderNode;
			return (
				folder.files.length === 0 && folder.db_instances.length === 0 && folder.folders.length === 0
			);
		}

		const children = 'children' in item ? item.children : null;
		const hasChildren = Array.isArray(children) && children.length > 0;

		return !hasChildren;
	});

	const isSelected = $derived($selectedItemsStore.has(item.id));

	// An item is a valid drop target if it's being hovered and not in the dragged selection
	const isValidDropTarget = $derived(
		$dragStateStore.isDragging &&
			!$dragStateStore.draggedItemIds.has(item.id) &&
			(onDragOver !== undefined || onDrop !== undefined) // Has drop handlers
	);

	// Highlight when this item is the currently hovered drop target
	const isHoveredDropTarget = $derived(
		$dragStateStore.hoveredTargetId === item.id && isValidDropTarget
	);

	// Virtual scroll visibility - simple Set lookup (untracked items won't be in visible set)
	const visible = $derived($visibleIdsStore.has(item.id));

	// Direct children for the visibility filter badge. Only databases and
	// schemas surface the badge; other items (tables, columns, type folders)
	// have noisy or trivial children and are excluded.
	const FILTERABLE_TYPES = new Set(['db_instance', 'schema']);
	const filterableChildren = $derived.by<{ id: string; name: string }[]>(() => {
		if (!FILTERABLE_TYPES.has(item.type)) return [];
		if (!('children' in item)) return [];
		const children = item.children ?? [];
		return children.map((c) => ({ id: c.id, name: c.name }));
	});
</script>

{#if visible}
	<Contextable {options} metadata={item} direction="right" style="height: 30px; display: flex;">
		{@const showActions = !($renamingItemIdStore === item.id) && actions && actions.length > 0}

		<div
			class="item"
			class:selected={isSelected}
			class:folder={item.type === 'folder'}
			class:database={item.type === 'db_instance'}
			class:hovered-target={isHoveredDropTarget}
			class:has-border-top={depth > 0}
			onclick={(e) => {
				if ($renamingItemIdStore === item.id) return;
				handleClick(item, e);
			}}
			role="presentation"
			data-id={item.id}
			data-depth={depth}
			data-parent-ids={parentIds.join(',')}
			{draggable}
			ondragstart={(e) => {
				if (item.type === 'file' || item.type === 'folder' || item.type === 'db_instance') {
					onDragStart?.(item as graph.FileNode | graph.FolderNode | graph.DBInstanceNode, e);
				}
			}}
			ondragover={(e) => {
				if (item.type === 'folder' || item.type === 'db_instance') {
					onDragOver?.(item as graph.FolderNode | graph.DBInstanceNode, e);
				}
			}}
			ondrop={(e) => {
				if (item.type === 'folder' || item.type === 'db_instance') {
					onDrop?.(item as graph.FolderNode | graph.DBInstanceNode, e);
				}
			}}
			ondragend={() => onDragEnd?.()}
		>
			<!-- eslint-disable-next-line @typescript-eslint/no-unused-vars -->
			{#each Array(depth) as _, i (i)}
				<div class="depth-spacer"></div>
			{/each}

			<div class="content">
				<div class="indicators-wrapper">
					<FieldIndicators {item} />
				</div>
				<ItemIcon {item} muted={muted()} />
				<div class="item-name-wrapper">
					<ItemName id={item.id} name={item.name} muted={muted()} type={item.type} />
					{#if filterableChildren.length > 0}
						<ChildVisibilityBadge parentId={item.id} items={filterableChildren} />
					{/if}
				</div>
			</div>

			<div class="badge-wrapper">
				<!-- eslint-disable-next-line @typescript-eslint/no-explicit-any -->
				{#each (item as any).badges || [] as badge, i (i)}
					<p class="badge">{badge}</p>
				{/each}
			</div>

			{#if showActions}
				<div class="action-wrapper">
					{#each actions as action, i (i)}
						<Button
							leftIcon={action.icon}
							emphasis="low"
							size="sm"
							iconSize={16}
							onclick={() => action.onClick(item)}
							noRadius
						/>
					{/each}
				</div>
			{/if}
		</div>
	</Contextable>
{:else}
	<div class="item-placeholder" data-id={item.id} data-parent-ids={parentIds.join(',')}></div>
{/if}

<style>
	.item {
		padding-right: var(--space-sm);
		padding-left: var(--space-sm);

		width: 100%;
		height: 30px;
		min-width: fit-content;
		display: inline-flex;
		align-items: stretch;

		border-bottom: var(--bw) transparent solid;

		transition: border-color 0.2s;
	}
	.item.has-border-top {
		border-top: var(--bw) transparent solid;
	}

	:global([data-depth='0'] > *:not(:first-child) .item[data-depth='0']) {
		border-top: var(--bw) transparent solid;
	}
	:global([data-depth='0'] > *:not(:first-child) .item[data-depth='0'].selected) {
		border-top-color: var(--border-color);
	}
	.item.selected {
		background: var(--gray-550) !important;
	}
	.item:not(.selected):hover {
		background: var(--gray-400) !important;
	}
	.item.selected.has-border-top {
		border-top-color: var(--border-color);
	}
	.item.selected {
		border-bottom-color: var(--border-color);
	}
	.item.hovered-target {
		background: var(--gray-0) !important;
	}
	.item-name-wrapper {
		display: flex;
		align-items: center;
		height: 100%;
	}

	:global(.item .name) {
		color: var(--gray-800);
	}
	:global(.item.selected .name),
	:global(.item:hover .name),
	:global(.item.hovered-target .name),
	:global(.item.selected .badge),
	:global(.item:hover .badge),
	:global(.item.hovered-target .badge) {
		color: var(--gray-1000) !important;
	}
	:global(.item.folder.selected svg),
	:global(.item.folder:hover svg),
	:global(.item.folder.hovered-target svg),
	:global(.item.database.selected svg),
	:global(.item.database:hover svg),
	:global(.item.database.hovered-target svg) {
		stroke: var(--gray-800) !important;
	}
	:global(.item.folder.selected .icon-folder-open svg),
	:global(.item.folder:hover .icon-folder-open svg),
	:global(.item.folder.hovered-target .icon-folder-open svg),
	:global(.item.database.selected .icon-folder-open svg),
	:global(.item.database:hover .icon-folder-open svg),
	:global(.item.database.hovered-target .icon-folder-open svg) {
		fill: var(--gray-700) !important;
	}

	.item .depth-spacer {
		display: inline-flex;
		padding-left: var(--space-sm);
		border-left-width: var(--bw);
		border-left-style: solid;
		border-left-color: var(--gray-0);
		margin-left: var(--space-sm);

		transition: border-left-color 0.3s ease-out;
	}
	.item.selected .depth-spacer {
		border-left-color: var(--gray-600) !important;
	}
	.item:hover .depth-spacer {
		border-left-color: var(--gray-550) !important;
	}

	.item .content {
		position: relative;
		width: 100%;
		display: flex;
		align-items: center;
		gap: var(--space-sm);
	}

	.item .badge-wrapper {
		display: flex;
		gap: var(--space-xs);
		position: sticky;
	}
	.item .badge-wrapper .badge {
		background-color: var(--gray-0);
		padding: var(--space-xxs) var(--space-xs);
		color: var(--gray-800);
		border-radius: var(--br-xs);
		height: fit-content;
		margin-top: 6px;
		font-size: var(--fs-xs);
	}

	.item .action-wrapper {
		display: none;
	}

	.item:hover .action-wrapper {
		display: flex;
		animation: action-wrapper-enter 160ms ease forwards;
		margin-left: var(--space-xs);
	}

	@keyframes action-wrapper-enter {
		from {
			opacity: 0;
			transform: translateX(4px);
		}

		to {
			opacity: 1;
			transform: translateX(0);
		}
	}

	.item-placeholder {
		height: 30px;
	}
	.item .indicators-wrapper {
		position: absolute;
		right: 100%;
		top: 50%;
		transform: translateY(-50%);
	}
</style>
