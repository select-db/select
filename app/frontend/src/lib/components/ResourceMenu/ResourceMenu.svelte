<script lang="ts">
	import Menu from '$lib/system/Menu/Menu.svelte';
	import ItemIcon from '$lib/components/views/shared/ItemIcon.svelte';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { FindFiles, newFileNode, type FileNode } from '$lib/wails/graph';
	import { layoutStore } from '$lib/components/Layout/layoutStore';
	import { recentItemsStore } from '$lib/stores/recentItemsStore';
	import { debounce } from '$lib/utils/debounce';
	import {
		flattenWorkspaceGraph,
		getOpenTabOptions,
		filterOptions,
		MAX_RESOURCE_MENU_OPTIONS
	} from './resourceMenuUtils';
	import type { ResourceMenuProps, ResourceMenuOption } from './types';
	import { getPathFromUri } from '../views/File/Header/utils';

	let {
		types,
		action,
		multiple = false,
		selectedIds = [],
		excludeIds = [],
		placeholder = 'Search files...',
		width = 400,
		maxHeight = 400,
		onClose,
		noBorder = false,
		extraOptions = [],
		searchScope,
		searchQuery = $bindable('')
	}: ResourceMenuProps = $props();

	/** Enough rows for the menu to rank; it shows the best MAX_RESOURCE_MENU_OPTIONS. */
	const FILE_RESULT_LIMIT = 200;

	const selectedSet = $derived(new Set(selectedIds));

	let debouncedQuery = $state('');

	const updateDebouncedQuery = debounce((query: string) => {
		debouncedQuery = query;
	}, 150);

	$effect(() => {
		updateDebouncedQuery(searchQuery);
	});

	// Each keystroke supersedes the one before it, cancelling the walk the
	// backend is still doing for it.
	let queriedFiles = $state<FileNode[]>([]);

	$effect(() => {
		const pattern = debouncedQuery.trim();
		if (!types.includes('file') || !pattern) {
			queriedFiles = [];
			return;
		}

		const query = FindFiles({ pattern, limit: FILE_RESULT_LIMIT });
		// A cancelled or failed query leaves the rows already shown alone rather
		// than blanking the menu under the cursor.
		query.then((files) => (queriedFiles = files)).catch(() => {});
		return () => query.cancel();
	});

	// Nothing typed, nothing to match on: the file rows are the recently opened
	// ones instead. Tabs and databases come from the layout and the graph below.
	const recentFiles = $derived.by(() => {
		if (debouncedQuery.trim() !== '') return [];
		return $recentItemsStore
			.filter((item) => item.type === 'file')
			.map((item) =>
				newFileNode({
					id: item.id,
					uri: item.uri,
					name: item.name,
					folder_id: item.folderId ?? ''
				})
			);
	});

	const graphOptions = $derived(
		flattenWorkspaceGraph($workspaceGraphStore, types, [...recentFiles, ...queriedFiles])
	);
	const tabOptions = $derived(getOpenTabOptions($layoutStore.root, types));

	const filteredOptions = $derived(
		filterOptions(
			graphOptions,
			tabOptions,
			$recentItemsStore,
			types,
			excludeIds,
			debouncedQuery,
			searchScope
		)
	);

	const filteredExtraOptions = $derived.by((): ResourceMenuOption[] => {
		if (!debouncedQuery) return extraOptions;
		const query = debouncedQuery.toLowerCase();
		return extraOptions.filter((opt) => opt.label.toLowerCase().includes(query));
	});

	const recentUrisForTypes = $derived(
		new Set($recentItemsStore.filter((item) => types.includes(item.type)).map((item) => item.uri))
	);
	const hasRecentSuggestions = $derived(
		filteredOptions.some((opt) => opt.uri && recentUrisForTypes.has(opt.uri))
	);

	const menuOptions = $derived.by((): MenuOption[] => {
		const merged = hasRecentSuggestions
			? [...filteredOptions, ...filteredExtraOptions]
			: [...filteredExtraOptions, ...filteredOptions];
		const allOptions = merged.slice(0, MAX_RESOURCE_MENU_OPTIONS);

		return allOptions.map((opt) => {
			const isSelected = selectedSet.has(opt.id);
			const base = {
				id: opt.id,
				label: opt.label,
				customData: opt,
				action: () => handleSelect(opt)
			};

			if (multiple) {
				return {
					...base,
					checkbox: true,
					checked: isSelected,
					onCheckClick: () => handleSelect(opt)
				};
			}

			return base;
		});
	});

	function handleSelect(option: ResourceMenuOption) {
		action(option);
		if (!multiple) onClose?.();
	}

	function uriToRelativePath(uri: string): string {
		if (!uri) return '';

		if (uri.startsWith('selectdb://')) return getPathFromUri(uri).join(' / ');

		return uri.split('/').filter(Boolean).join(' / ');
	}

	function getPath(option: ResourceMenuOption): string | undefined {
		if ('path' in option.node && option.node.path) {
			return option.node.path;
		}
		if ('uri' in option.node && option.node.uri) {
			return uriToRelativePath(option.node.uri);
		}
		return undefined;
	}
</script>

<Menu
	options={menuOptions}
	bind:searchQuery
	searchEnabled
	externalFilter
	searchPlaceholder={placeholder}
	{width}
	{maxHeight}
	minHeight={maxHeight}
	{onClose}
	{noBorder}
	emptyMessage="No items found"
	noResultsMessage="No matching items"
>
	{#snippet optionContent(data)}
		{@const option = data as ResourceMenuOption}
		{@const path = getPath(option)}
		<div class="option-row">
			<ItemIcon item={option.node} noDepth />
			<span class="option-label">{option.label}</span>
			{#if path}
				<span class="option-path">{path}</span>
			{/if}
		</div>
	{/snippet}
</Menu>

<style>
	.option-row {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		width: 100%;
		overflow: hidden;
		height: 30px;
	}

	.option-label {
		flex-shrink: 0;
		font-size: var(--fs-sm);
		color: var(--gray-800);
	}

	.option-path {
		flex: 1;
		min-width: 0;
		font-size: var(--fs-xs);
		color: var(--gray-800);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
