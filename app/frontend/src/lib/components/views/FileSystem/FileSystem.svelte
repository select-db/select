<script lang="ts">
	import Contextable from '$lib/system/ContextMenu/Contextable.svelte';
	import { scrollShadow } from '$lib/actions/scrollShadow';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { rootOptions } from './Files/options/rootOptions';
	import FileItems from './Files/FileItems.svelte';
	import { createDragAndDropHandlers } from './Files/helpers/dragAndDropHandlers';
	import {
		expandedItemIdsStore,
		focusedFsItemStore,
		setFocusedFsItem,
		fsPanelFocusSignal,
		setItemSelection,
		toggleIsItemExpanded,
		expandItem,
		renamingItemIdStore
	} from '$lib/components/views/shared/sharedStore';
	import {
		buildVisibilityIndex,
		updateScrollWindow,
		fsVisibilityIndexStore
	} from './Files/helpers/visibilityStore';
	import { hiddenChildrenStore } from './Files/helpers/childVisibilityStore';

	let scrollContainer: HTMLDivElement;
	let firstItem: HTMLElement | null = null;
	let firstItemParentIds: string | null = null;

	let ticking = false;

	const ITEM_HEIGHT = 30;

	// Focus the scroll container whenever an item is clicked in the panel
	$effect(() => {
		if ($fsPanelFocusSignal > 0) scrollContainer?.focus({ preventScroll: true });
	});

	// Return focus to the scroll container when rename mode ends
	let prevRenamingId: string | null = null;
	$effect(() => {
		const current = $renamingItemIdStore;
		if (prevRenamingId !== null && current === null) {
			scrollContainer?.focus({ preventScroll: true });
		}
		prevRenamingId = current;
	});

	const scrollToIndex = (index: number) => {
		if (!scrollContainer) return;
		const top = index * ITEM_HEIGHT;
		const bottom = top + ITEM_HEIGHT;
		if (top < scrollContainer.scrollTop) {
			scrollContainer.scrollTop = top;
		} else if (bottom > scrollContainer.scrollTop + scrollContainer.clientHeight) {
			scrollContainer.scrollTop = bottom - scrollContainer.clientHeight;
		}
	};

	// Find the closest visible ancestor of `id` using descendantRange
	const findParentInFlatList = (id: string): string | null => {
		const { flatIds, descendantRange } = $fsVisibilityIndexStore;
		const idx = flatIds.indexOf(id);
		if (idx === -1) return null;
		for (let i = idx - 1; i >= 0; i--) {
			const range = descendantRange.get(flatIds[i]);
			if (range && range[0] <= idx && idx <= range[1]) return flatIds[i];
		}
		return null;
	};

	const handleKeydown = (e: KeyboardEvent) => {
		// Don't handle navigation while editing a
		// file/folder name (rename input is active)
		if ($renamingItemIdStore) return;

		const focusedId = $focusedFsItemStore;
		if (!focusedId) return;

		const { flatIds } = $fsVisibilityIndexStore;
		const idx = flatIds.indexOf(focusedId);

		switch (e.key) {
			case 'Enter': {
				e.preventDefault();
				setTimeout(() => renamingItemIdStore.set(focusedId), 0);
				break;
			}
			case 'ArrowDown': {
				e.preventDefault();
				if (idx === -1 || idx >= flatIds.length - 1) break;
				const nextId = flatIds[idx + 1];
				setItemSelection([nextId]);
				setFocusedFsItem(nextId);
				scrollToIndex(idx + 1);
				break;
			}
			case 'ArrowUp': {
				e.preventDefault();
				if (idx <= 0) break;
				const prevId = flatIds[idx - 1];
				setItemSelection([prevId]);
				setFocusedFsItem(prevId);
				scrollToIndex(idx - 1);
				break;
			}
			case 'ArrowRight': {
				e.preventDefault();
				const isExpanded = $expandedItemIdsStore.get(focusedId) === true;
				if (!isExpanded) {
					expandItem(focusedId);
				} else if (idx + 1 < flatIds.length) {
					const childId = flatIds[idx + 1];
					setItemSelection([childId]);
					setFocusedFsItem(childId);
					scrollToIndex(idx + 1);
				}
				break;
			}
			case 'ArrowLeft': {
				e.preventDefault();
				const isExpanded = $expandedItemIdsStore.get(focusedId) === true;
				if (isExpanded) {
					toggleIsItemExpanded(focusedId);
				} else {
					const parentId = findParentInFlatList(focusedId);
					if (parentId) {
						setItemSelection([parentId]);
						setFocusedFsItem(parentId);
						const parentIdx = flatIds.indexOf(parentId);
						if (parentIdx !== -1) scrollToIndex(parentIdx);
					}
				}
				break;
			}
		}
	};

	const handleScroll = () => {
		if (!ticking) {
			ticking = true;
			requestAnimationFrame(() => {
				runScrollLogic();
				ticking = false;
			});
		}
	};

	const getStickyTarget = (id: string): HTMLElement | null => {
		const item = document.querySelector(`.item[data-id="${id}"]`) as HTMLElement | null;
		if (!item) return null;
		return (item.parentElement as HTMLElement) ?? null;
	};

	const runScrollLogic = () => {
		if (!scrollContainer) return;

		updateScrollWindow('fs', scrollContainer.scrollTop, scrollContainer.clientHeight);

		const containerRect = scrollContainer.getBoundingClientRect();
		const items = Array.from(scrollContainer.querySelectorAll('.item')) as HTMLElement[];

		if (scrollContainer.scrollTop === 0) {
			document.querySelectorAll('.sticky').forEach((el) => {
				el.classList.remove('sticky', 'last');
				(el as HTMLElement).style.top = '';
			});
			firstItem = firstItemParentIds = null;
			return;
		}

		firstItem =
			items.find((item) => {
				// Skip items whose Contextable wrapper is currently sticky (would self-match)
				if (item.parentElement?.classList.contains('sticky')) return false;

				const offset = (item.getAttribute('data-parent-ids')?.split(',') || []).reduce(
					(sum, id) => {
						const el = getStickyTarget(id);
						return sum + (el?.offsetHeight || 0);
					},
					0
				);

				const rect = item.getBoundingClientRect();
				return rect.top >= containerRect.top + offset && rect.top < containerRect.bottom;
			}) || null;

		firstItemParentIds = firstItem?.getAttribute('data-parent-ids') || null;

		const newStickyIds = new Set(firstItemParentIds?.split(',') || []);

		document.querySelectorAll('.sticky').forEach((el) => {
			const id = el.getAttribute('data-sticky-id');
			if (!id || !newStickyIds.has(id)) {
				el.classList.remove('sticky', 'last');
				(el as HTMLElement).style.top = '';
				el.removeAttribute('data-sticky-id');
			}
		});

		let topOffset = 0;
		const parentIds = firstItemParentIds?.split(',') || [];
		let lastStickyElement: HTMLElement | null = null;

		for (let i = 0; i < parentIds.length; i++) {
			const id = parentIds[i];
			const el = getStickyTarget(id);
			if (el) {
				el.classList.add('sticky');
				el.classList.remove('last');
				el.style.top = `${topOffset}px`;
				el.setAttribute('data-sticky-id', id);
				topOffset += el.offsetHeight;
				lastStickyElement = el;
			}
		}

		if (lastStickyElement) {
			lastStickyElement.classList.add('last');
		}
	};

	const root = $derived($workspaceGraphStore?.folders[0] ?? null);

	// Create drop handlers for root folder
	const { handleDragOver, handleDrop, handleDragLeave } = createDragAndDropHandlers('fs');

	// Rebuild visibility index when tree, expanded state, or hidden children change
	$effect(() => {
		if (!root) return;
		if (!scrollContainer) return;

		const expandedIds = $expandedItemIdsStore;
		const hidden = $hiddenChildrenStore;
		buildVisibilityIndex(
			'fs',
			root.folders,
			root.files,
			root.db_instances,
			[],
			expandedIds,
			hidden
		);

		// Update scroll window immediately after building index
		updateScrollWindow('fs', scrollContainer.scrollTop, scrollContainer.clientHeight);
	});
</script>

<Contextable
	options={rootOptions}
	metadata={root}
	direction="right"
	style="
		display: flex;
		flex-grow: 1;
		height: 100%;
		overflow-y: scroll;
		overflow-x: hidden;
	"
>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="scrollable no-scrollbar"
		bind:this={scrollContainer}
		use:scrollShadow
		onscroll={handleScroll}
		onkeydown={handleKeydown}
		ondragover={(e) => {
			if (root) handleDragOver(root, e);
		}}
		ondrop={(e) => {
			if (root) handleDrop(root, e);
		}}
		ondragleave={handleDragLeave}
		role="region"
		aria-label="File system root drop zone"
		tabindex="-1"
	>
		{#if root}
			<FileItems
				folders={root.folders}
				files={root.files}
				databases={root.db_instances}
				databaseItems={[]}
			/>
		{/if}
	</div>
</Contextable>

<style>
	.scrollable {
		flex-grow: 1;
		height: 100%;
		overflow-y: scroll;
		overflow-x: hidden;
		outline: none;
	}

	:global(.leftbar .sticky) {
		position: sticky;
		z-index: 1;
	}

	:global(.leftbar .sticky .item) {
		background-color: var(--gray-0);
	}
	:global(.leftbar .sticky .item:hover) {
		background: var(--gray-300);
	}

	:global(.leftbar .sticky.last .item) {
		border-bottom: var(--border) !important;
		box-shadow: var(--shadow) 0px 20px 20px -10px;
	}
</style>
