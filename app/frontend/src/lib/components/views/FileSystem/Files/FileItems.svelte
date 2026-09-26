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
	import {
		clickDatasource,
		createClickHandlers,
		createClickGestureHandlers
	} from './helpers/clickHandlers';
	import { hiddenChildrenStore, filterVisibleChildren } from './helpers/childVisibilityStore';
	import { expandableItemTypes } from '$lib/components/views/shared/expandableItemTypes';
	import { navigateToFile } from '$lib/components/views/shared/navigateToFile';
	import { navigateToSchema } from '$lib/components/views/Schema/navigateToSchema';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
	import { get } from 'svelte/store';
	import { untrack } from 'svelte';

	let {
		files,
		folders,
		datasources,
		datasourceItems,
		depth = 0,
		parentIds = [],
		ctx = 'fs',
		insideDatasource = false
	}: {
		files: graph.FileNode[];
		folders: graph.FolderNode[];
		datasources: graph.DatasourceNode[];
		datasourceItems: graph.DatasourceItemNode[];
		depth?: number;
		parentIds?: string[];
		ctx?: 'fs' | 'git' | 'search';
		insideDatasource?: boolean;
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
	const { handleFolderClick, handleFileClick, handleDatasourceClick } = untrack(() =>
		createClickHandlers(ctx, lastClickedId)
	);

	type AnyItem =
		| graph.FolderNode
		| graph.DatasourceNode
		| graph.DatasourceItemNode
		| graph.FileNode;

	// Simple click handler for items inside database (no selection support)
	const handleSimpleClick = (item: AnyItem) => {
		if (item.type === 'file') {
			const file = item as graph.FileNode;
			// Special handling for schema.sql files inside database folders
			if (file.name === 'schema.sql' && file.folder_id) {
				const workspace = get(workspaceGraphStore);
				const datasource = (workspace?.datasources ?? []).find((db) => db.uri === file.folder_id);
				if (datasource) {
					void navigateToSchema(datasource.id);
					return;
				}
			}
			navigateToFile(file);
			return;
		}

		// A row that does not expand has no other use for a click, so it runs
		// whichever of its options asks for the gesture -- "Infos..." on a
		// database item today. Nothing carries the flag in the menu a
		// multi-selection gets, so a click there stays as inert as it was.
		if (!expandableItemTypes.has(item.type)) {
			runOption(clickOption(item), item);
			return;
		}

		// Through clickDatasource, so a database inside another folder answers a
		// click the way one at the root does: opening while it is empty, and
		// reading the schema that makes it not be.
		if (item.type === 'datasource') {
			clickDatasource(item as graph.DatasourceNode);
			return;
		}

		toggleIsItemExpanded(item.id);
	};

	// Route a click to the handler that item kind would have got on its own.
	const clickItem = (item: AnyItem, event?: MouseEvent) => {
		if (insideDatasource) return handleSimpleClick(item);
		if (item.type === 'folder') return handleFolderClick(item as graph.FolderNode, event);
		if (item.type === 'file') return handleFileClick(item as graph.FileNode, event);
		return handleDatasourceClick(item as graph.DatasourceNode, event);
	};

	// Clicking and double-clicking run the options that declare the gesture, so
	// it is written next to the action itself instead of listed again here.
	const clickOption = (item: AnyItem): ContextMenuOption | undefined =>
		getOptions(item, ctx).find((option) => option.runOnClick);

	const doubleClickOption = (item: AnyItem): ContextMenuOption | undefined =>
		getOptions(item, ctx).find((option) => option.runOnDoubleClick);

	// An option is written for a menu, which hands it the callback that closes
	// itself. Fired from a gesture there is no menu to close.
	const runOption = (option: ContextMenuOption | undefined, item: AnyItem) =>
		option?.action?.(() => {}, item);

	// Items that toggle open on click ignore the second click of a double-click,
	// so a double-click doesn't expand and collapse them on the way to its own
	// action. The rest keep reacting to every click.
	const { handleClick, handleDoubleClick } = createClickGestureHandlers<AnyItem>({
		shouldDefer: (item) => expandableItemTypes.has(item.type) && !!doubleClickOption(item),
		onClick: clickItem,
		onDoubleClick: (item) => runOption(doubleClickOption(item), item)
	});

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
	{#each datasources as datasource (datasource.id)}
		<ItemDisplay
			{depth}
			{parentIds}
			{draggable}
			{handleClick}
			{handleDoubleClick}
			item={datasource}
			options={() => getOptions(datasource)}
			actions={getActions({ item: datasource })}
			onDragStart={handleDragStart}
			onDragOver={(item, event) => {
				if (isEventFromExtendedContent(datasource.id, event)) return;
				handleDragOver(item, event);
			}}
			onDrop={async (item, event) => {
				if (isEventFromExtendedContent(datasource.id, event)) return;
				await handleDrop(item, event);
			}}
			onDragEnd={handleDragEnd}
		/>

		{#if isExpanded(datasource.id)}
			<div
				class="folder-content-drop-zone"
				class:drop-zone-hovered={$dragStateStore.hoveredTargetId === datasource.id}
				data-drop-zone={datasource.id}
				ondragover={(e) => handleDragOver(datasource, e)}
				ondrop={async (e) => await handleDrop(datasource, e)}
				role="region"
				aria-label={`${datasource.name} content drop zone`}
			>
				<FileItems
					depth={depth + 1}
					folders={datasource.folders}
					datasources={[]}
					datasourceItems={filterVisibleChildren(
						datasource.id,
						datasource.children,
						$hiddenChildrenStore
					)}
					files={datasource.files}
					parentIds={[...parentIds, datasource.id]}
					{ctx}
					insideDatasource={true}
				/>
			</div>
		{/if}
	{/each}

	{#each datasourceItems as item (item.id)}
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
				datasources={[]}
				files={[]}
				datasourceItems={filterVisibleChildren(item.id, item.children, $hiddenChildrenStore)}
				depth={depth + 1}
				parentIds={[...parentIds, item.id]}
				{ctx}
				insideDatasource={true}
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
					datasources={folder.datasources}
					datasourceItems={[]}
					depth={depth + 1}
					parentIds={[...parentIds, folder.id]}
					{ctx}
					{insideDatasource}
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
	:global(.drop-zone-hovered .item.datasource svg) {
		stroke: var(--gray-800) !important;
	}
	:global(.drop-zone-hovered .item.folder .icon-folder-open svg),
	:global(.drop-zone-hovered .item.datasource .icon-folder-open svg) {
		fill: var(--gray-700) !important;
	}
</style>
