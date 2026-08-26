<script lang="ts">
	import FileItems from './FileItems.svelte';
	import { getOptions } from './options/getOptions';
	import ItemDisplay from '$lib/components/views/shared/ItemDisplay.svelte';
	import * as graph from '$lib/wails/graph';
	import {
		expandedItemIdsStore,
		dragStateStore,
		toggleIsItemExpanded
	} from '$lib/components/views/shared/sharedStore';
	import { getActions } from './actions/getActions';
	import { createDragAndDropHandlers } from './helpers/dragAndDropHandlers';
	import { createClickHandlers, createDeferredClickHandlers } from './helpers/clickHandlers';
	import { hiddenChildrenStore, filterVisibleChildren } from './helpers/childVisibilityStore';
	import { loadSchema } from '$lib/utils/query/loadSchema';
	import { expandableItemTypes } from '$lib/components/views/shared/expandableItemTypes';
	import { navigateToFile } from '$lib/components/views/shared/navigateToFile';
	import { navigateToSchema } from '$lib/components/views/Schema/navigateToSchema';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
	import { get } from 'svelte/store';
	import { onDestroy, untrack } from 'svelte';

	let {
		files,
		folders,
		databases,
		databaseItems,
		depth = 0,
		parentIds = [],
		ctx = 'fs',
		insideDatabase = false
	}: {
		files: graph.FileNode[];
		folders: graph.FolderNode[];
		databases: graph.DBInstanceNode[];
		databaseItems: graph.DBInstanceItemNode[];
		depth?: number;
		parentIds?: string[];
		ctx?: 'fs' | 'git' | 'search';
		insideDatabase?: boolean;
	} = $props();

	// Track last clicked item for shift+click range selection (using object ref for factory functions)
	const lastClickedId = { current: null as string | null };

	// Create drag and drop handlers (includes auto-expand)
	// The context a tree is rendered in is fixed for its lifetime, so the
	// handlers are built once from it.
	const { handleDragStart, handleDragOver, handleDrop, handleDragEnd } = untrack(() =>
		createDragAndDropHandlers(ctx)
	);

	// Create click and selection handlers (used when not inside database)
	const { handleFolderClick, handleFileClick, handleDatabaseClick } = untrack(() =>
		createClickHandlers(ctx, lastClickedId)
	);

	type AnyItem =
		| graph.FolderNode
		| graph.DBInstanceNode
		| graph.DBInstanceItemNode
		| graph.FileNode;

	// Simple click handler for items inside database (no selection support)
	const handleSimpleClick = (item: AnyItem) => {
		if (item.type === 'file') {
			const file = item as graph.FileNode;
			// Special handling for schema.sql files inside database folders
			if (file.name === 'schema.sql' && file.folder_id) {
				const workspace = get(workspaceGraphStore);
				const database = (workspace?.db_instances ?? []).find((db) => db.uri === file.folder_id);
				if (database) {
					void navigateToSchema(database.id);
					return;
				}
			}
			navigateToFile(file);
			return;
		}

		if (!expandableItemTypes.has(item.type)) return;

		toggleIsItemExpanded(item.id);

		// Through loadSchema, not QuerySchema directly: expanding a database is the
		// most common way to load a schema, and it was the one path that dropped
		// the result on the floor — no await, no catch. A server that refuses the
		// read left the node expanded and empty with nothing said anywhere.
		if (item.type === 'db_instance' && 'children' in item && item.children.length === 0) {
			void loadSchema({ database: item as graph.DBInstanceNode });
		}
	};

	// Route a click to the handler that item kind would have got on its own.
	const clickItem = (item: AnyItem, event?: MouseEvent) => {
		if (insideDatabase) return handleSimpleClick(item);
		if (item.type === 'folder') return handleFolderClick(item as graph.FolderNode, event);
		if (item.type === 'file') return handleFileClick(item as graph.FileNode, event);
		return handleDatabaseClick(item as graph.DBInstanceNode, event);
	};

	// Double-clicking runs the item's runOnDoubleClick option, so the gesture is
	// declared next to the action itself instead of listed again here.
	const doubleClickOption = (item: AnyItem): ContextMenuOption | undefined =>
		getOptions(item, ctx).find((option) => option.runOnDoubleClick);

	// Items that toggle open on click hold that click back for the double-click
	// window, so a double-click doesn't expand and collapse them on the way. The
	// rest act on the first click, where the delay would only feel slow.
	const {
		handleClick,
		handleDoubleClick,
		cancel: cancelPendingClick
	} = createDeferredClickHandlers<AnyItem>({
		shouldDefer: (item) => expandableItemTypes.has(item.type) && !!doubleClickOption(item),
		onClick: clickItem,
		onDoubleClick: (item) => doubleClickOption(item)?.action?.(() => {}, item)
	});

	onDestroy(cancelPendingClick);

	const isExpanded = (id: string): boolean => {
		const store = $expandedItemIdsStore;
		return store instanceof Map ? store.get(id) === true : false;
	};

	const isEventFromExtendedContent = (itemId: string, e: DragEvent) => {
		const target = e.target as HTMLElement;
		return target.closest(`[data-drop-zone="${itemId}"]`);
	};

	const draggable = $derived(ctx === 'fs');
</script>

<div data-depth={depth}>
	{#each databases as database (database.id)}
		<ItemDisplay
			{depth}
			{parentIds}
			{draggable}
			{handleClick}
			{handleDoubleClick}
			item={database}
			options={() => getOptions(database)}
			actions={getActions({ item: database })}
			onDragStart={handleDragStart}
			onDragOver={(item, event) => {
				if (isEventFromExtendedContent(database.id, event)) return;
				handleDragOver(item, event);
			}}
			onDrop={async (item, event) => {
				if (isEventFromExtendedContent(database.id, event)) return;
				await handleDrop(item, event);
			}}
			onDragEnd={handleDragEnd}
		/>

		{#if isExpanded(database.id)}
			<div
				class="folder-content-drop-zone"
				class:drop-zone-hovered={$dragStateStore.hoveredTargetId === database.id}
				data-drop-zone={database.id}
				ondragover={(e) => handleDragOver(database, e)}
				ondrop={async (e) => await handleDrop(database, e)}
				role="region"
				aria-label={`${database.name} content drop zone`}
			>
				<FileItems
					depth={depth + 1}
					folders={database.folders}
					databases={[]}
					databaseItems={filterVisibleChildren(
						database.id,
						database.children,
						$hiddenChildrenStore
					)}
					files={database.files}
					parentIds={[...parentIds, database.id]}
					{ctx}
					insideDatabase={true}
				/>
			</div>
		{/if}
	{/each}

	{#each databaseItems as item (item.id)}
		<ItemDisplay
			{depth}
			{item}
			{parentIds}
			{handleClick}
			{handleDoubleClick}
			options={() => getOptions(item)}
		/>

		{#if isExpanded(item.id)}
			<FileItems
				folders={[]}
				databases={[]}
				files={[]}
				databaseItems={filterVisibleChildren(item.id, item.children, $hiddenChildrenStore)}
				depth={depth + 1}
				parentIds={[...parentIds, item.id]}
				{ctx}
				insideDatabase={true}
			/>
		{/if}
	{/each}

	{#each folders as folder (folder.id)}
		<ItemDisplay
			{depth}
			{parentIds}
			{draggable}
			item={folder}
			{handleClick}
			{handleDoubleClick}
			options={() => getOptions(folder, ctx)}
			actions={getActions({ item: folder, ctx })}
			onDragStart={handleDragStart}
			onDragOver={(item, event) => {
				if (isEventFromExtendedContent(folder.id, event)) return;
				handleDragOver(item, event);
			}}
			onDrop={async (item, event) => {
				if (isEventFromExtendedContent(folder.id, event)) return;
				await handleDrop(item, event);
			}}
			onDragEnd={handleDragEnd}
		/>

		{#if isExpanded(folder.id)}
			<div
				class="folder-content-drop-zone"
				class:drop-zone-hovered={$dragStateStore.hoveredTargetId === folder.id}
				data-drop-zone={folder.id}
				ondragover={(e) => handleDragOver(folder, e)}
				ondrop={async (e) => await handleDrop(folder, e)}
				role="region"
				aria-label={`${folder.name} content drop zone`}
			>
				<FileItems
					files={folder.files}
					folders={folder.folders}
					databases={folder.db_instances}
					databaseItems={[]}
					depth={depth + 1}
					parentIds={[...parentIds, folder.id]}
					{ctx}
					{insideDatabase}
				/>
			</div>
		{/if}
	{/each}

	{#each files as file (file.id)}
		<ItemDisplay
			{depth}
			{parentIds}
			{draggable}
			item={file}
			{handleClick}
			{handleDoubleClick}
			options={() => getOptions(file, ctx)}
			actions={getActions({ item: file, ctx })}
			onDragStart={handleDragStart}
			onDragEnd={handleDragEnd}
		/>
	{/each}
</div>

<style>
	.folder-content-drop-zone {
		display: contents;
	}

	/* Highlight all items within a hovered drop zone */
	:global(.drop-zone-hovered .item) {
		background: var(--gray-300) !important;
	}

	:global(.drop-zone-hovered .item .name),
	:global(.drop-zone-hovered .item .badge) {
		color: var(--gray-1000) !important;
	}

	:global(.drop-zone-hovered .item.folder svg),
	:global(.drop-zone-hovered .item.database svg) {
		stroke: var(--gray-800) !important;
	}
	:global(.drop-zone-hovered .item.folder .icon-folder-open svg),
	:global(.drop-zone-hovered .item.database .icon-folder-open svg) {
		fill: var(--gray-700) !important;
	}
</style>
