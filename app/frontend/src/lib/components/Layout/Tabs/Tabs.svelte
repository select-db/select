<script lang="ts">
	import { scale } from 'svelte/transition';
	import { flip } from 'svelte/animate';
	import { tick } from 'svelte';

	import Button from '$lib/system/Button/Button.svelte';
	import Contextable from '$lib/system/ContextMenu/Contextable.svelte';

	import ItemIcon from '$lib/components/views/shared/ItemIcon.svelte';
	import {
		removeTab,
		reorderTabs,
		setActiveTab,
		moveTabToGroup,
		layoutStore,
		canNavigateToPreviousTab,
		canNavigateToNextTab,
		navigateToPreviousTab,
		navigateToNextTab,
		type Tab,
		getTabLabel
	} from '$lib/components/Layout/layoutStore';

	import { tabOptions } from './tabOptions';
	import { dragState } from './tabDragState.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import TabActions from './TabActions.svelte';
	import { executeCommand } from '$lib/stores/commandRegistry';
	import { scrollShadow } from '$lib/actions/scrollShadow';

	type Props = {
		tabs: Tab[];
		groupId: string;
		activeTabId: string | null;
	};

	let { tabs, groupId, activeTabId }: Props = $props();

	let insertPosition: number | null = $state(null);
	let wrapperElement: HTMLElement | null = $state(null);
	let cancelNextDrag = false;
	let lastScrolledTabId: string | null = null;

	// Access global drag state
	const draggedTabId = $derived(dragState.tabId);
	const draggedFromGroupId = $derived(dragState.groupId);

	const isActiveGroup = $derived($layoutStore.activeGroupId === groupId);

	const handleDragStart = (tab: Tab) => (e: DragEvent) => {
		dragState.tabId = tab.id;
		dragState.groupId = groupId;
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
			e.dataTransfer.setData('application/x-tab-id', tab.id);
			e.dataTransfer.setData('application/x-group-id', groupId);
		}
	};

	const handleDragOver = (index: number) => (e: DragEvent) => {
		// Check if this is a tab drag
		const isTabDrag = e.dataTransfer?.types.includes('application/x-tab-id');
		if (!isTabDrag) return;

		e.preventDefault();
		e.stopPropagation();

		if (e.dataTransfer) {
			e.dataTransfer.dropEffect = 'move';
		}

		// Track which group is being hovered
		dragState.hoveredGroupId = groupId;

		// Determine insert position based on mouse position
		const target = e.currentTarget as HTMLElement;
		const rect = target.getBoundingClientRect();
		const mouseX = e.clientX;
		const elementCenter = rect.left + rect.width / 2;

		insertPosition = mouseX < elementCenter ? index : index + 1;
	};

	const handleDrop = (index: number) => (e: DragEvent) => {
		e.preventDefault();
		e.stopPropagation();

		const droppedTabId = e.dataTransfer?.getData('application/x-tab-id');
		const sourceGroupId = e.dataTransfer?.getData('application/x-group-id');

		if (!droppedTabId || !sourceGroupId) return;

		// Recalculate insert position at drop time
		const target = e.currentTarget as HTMLElement;
		const rect = target.getBoundingClientRect();
		const mouseX = e.clientX;
		const elementCenter = rect.left + rect.width / 2;
		const finalPosition = mouseX < elementCenter ? index : index + 1;

		if (sourceGroupId === groupId) {
			// Same group - reorder
			const draggedIndex = tabs.findIndex((t) => t.id === droppedTabId);
			if (
				draggedIndex !== -1 &&
				draggedIndex !== finalPosition &&
				draggedIndex !== finalPosition - 1
			) {
				let adjustedPosition = finalPosition;
				if (draggedIndex < finalPosition) {
					adjustedPosition = finalPosition - 1;
				}
				reorderTabs(groupId, draggedIndex, adjustedPosition);
			}
		} else {
			// Different group - move tab at insert position
			moveTabToGroup(droppedTabId, groupId, finalPosition);
		}

		insertPosition = null;
		dragState.tabId = null;
		dragState.groupId = null;
		dragState.hoveredGroupId = null;
	};

	const handleDragEnd = () => {
		insertPosition = null;
		dragState.tabId = null;
		dragState.groupId = null;
		dragState.hoveredGroupId = null;
	};

	const handleWrapperDragOver = (e: DragEvent) => {
		const droppedTabId = e.dataTransfer?.types.includes('application/x-tab-id');
		if (!droppedTabId) return;

		e.preventDefault();
		e.stopPropagation();

		// Track which group is being hovered
		dragState.hoveredGroupId = groupId;

		// Default: insert at end when hovering over wrapper
		insertPosition = tabs.length;
	};

	const handleWrapperDrop = (e: DragEvent) => {
		e.preventDefault();
		e.stopPropagation();

		const droppedTabId = e.dataTransfer?.getData('application/x-tab-id');
		const sourceGroupId = e.dataTransfer?.getData('application/x-group-id');

		if (!droppedTabId || !sourceGroupId || insertPosition === null) return;

		if (sourceGroupId === groupId) {
			// Same group - reorder to end
			const draggedIndex = tabs.findIndex((t) => t.id === droppedTabId);
			if (draggedIndex !== -1) {
				let finalIndex = insertPosition;
				if (draggedIndex < insertPosition) {
					finalIndex = insertPosition - 1;
				}
				reorderTabs(groupId, draggedIndex, finalIndex);
			}
		} else {
			// Different group - move tab at insert position
			moveTabToGroup(droppedTabId, groupId, insertPosition);
		}

		insertPosition = null;
		dragState.tabId = null;
		dragState.groupId = null;
		dragState.hoveredGroupId = null;
	};

	// Scroll active tab into view when it changes.
	// lastScrolledTabId guards against spurious re-runs (e.g. from pointerdown store updates)
	// that fire with the same activeTabId and would scroll the old active tab back into view.
	$effect(() => {
		if (!activeTabId || !wrapperElement) return;
		if (activeTabId === lastScrolledTabId) return;
		lastScrolledTabId = activeTabId;

		const wrapper = wrapperElement;

		tick().then(() => {
			const activeTabElement = wrapper.querySelector('.tab.active') as HTMLElement;
			if (!activeTabElement) return;

			const wrapperRect = wrapper.getBoundingClientRect();
			const tabRect = activeTabElement.getBoundingClientRect();
			const scrollOffset =
				tabRect.left + tabRect.width / 2 - (wrapperRect.left + wrapperRect.width / 2);

			wrapper.scrollBy({ left: scrollOffset, behavior: 'smooth' });
		});
	});
</script>

{#if tabs.length}
	<div class="tabs-container">
		<div class="tab-nav-arrows" aria-label="Tab navigation">
			<Button
				size="sm"
				emphasis="low"
				leftIcon="arrow-left"
				iconSize={20}
				onclick={() => navigateToPreviousTab(groupId)}
				disabled={!canNavigateToPreviousTab(groupId)}
				style="padding-left: var(--space-sm);"
				noBounce
			/>
			<Button
				size="sm"
				emphasis="low"
				leftIcon="arrow-right"
				iconSize={20}
				onclick={() => navigateToNextTab(groupId)}
				disabled={!canNavigateToNextTab(groupId)}
				style="padding-right: var(--space-sm);"
				noBounce
			/>
		</div>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="wrapper no-scrollbar"
			bind:this={wrapperElement}
			use:scrollShadow={{ axis: 'x', top: true }}
			ondragover={handleWrapperDragOver}
			ondrop={handleWrapperDrop}
		>
			{#each tabs as tab, index (tab.id)}
				{@const active = tab.id === activeTabId}
				{@const isDragged = draggedTabId === tab.id && draggedFromGroupId === groupId}
				{@const showDropIndicator =
					insertPosition === index && draggedTabId !== null && dragState.hoveredGroupId === groupId}

				<div
					class="tab-container"
					in:scale={{ duration: 150, start: 0.9, opacity: 0 }}
					out:scale={{ duration: 150, start: 1, opacity: 0 }}
					animate:flip={{ duration: 150 }}
				>
					{#if showDropIndicator}
						<div class="drop-indicator"></div>
					{/if}

					<Contextable options={tabOptions} metadata={tab} direction="right" style="display: flex;">
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<!-- svelte-ignore a11y_click_events_have_key_events -->
						<div
							class="tab"
							class:active
							class:dragging={isDragged}
							draggable="true"
							onmousedown={() => {
								cancelNextDrag = false;
							}}
							ondragstart={(e) => {
								if (cancelNextDrag) {
									cancelNextDrag = false;
									e.preventDefault();
									return;
								}
								handleDragStart(tab)(e);
							}}
							ondragover={handleDragOver(index)}
							ondrop={handleDrop(index)}
							ondragend={handleDragEnd}
							onclick={(e) => {
								e.preventDefault();
								e.stopPropagation();
								setActiveTab(groupId, tab.id);
							}}
						>
							{#if tab.file}
								<ItemIcon item={tab.file.node} noDepth />
							{:else if tab.database}
								<ItemIcon item={tab.database.node} noDepth />
							{:else if tab.schema}
								<Icon icon="schema" size={18} />
							{:else if tab.diff}
								<Icon icon="diff" size={18} />
							{:else if tab.terminal}
								<Icon icon="terminal" size={18} />
							{:else if tab.chat}
								<Icon icon="chat" size={17} />
							{:else if tab.settings}
								<Icon icon="cog" size={18} />
							{/if}
							<p class="name">{getTabLabel(tab)}</p>

							<Button
								size="sm"
								emphasis="low"
								leftIcon="cross"
								iconSize={13}
								onmousedown={() => {
									cancelNextDrag = true;
								}}
								onclick={() => removeTab(tab.id)}
								classes="closeBtn"
								noBounce
							/>

							{#if active}
								<!-- Concave corners flaring the active tab into the content surface -->
								<span class="flare flare-left"></span>
								<span class="flare flare-right"></span>
							{/if}
						</div>
					</Contextable>
				</div>
			{/each}

			{#if insertPosition === tabs.length && draggedTabId !== null && dragState.hoveredGroupId === groupId}
				<div class="drop-indicator"></div>
			{/if}

			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<div
				class="tab new-tab"
				title="New SQL file"
				onclick={() => executeCommand('workbench.newSqlFile')}
			>
				<Icon icon="plus" size={16} />
			</div>

			<div class="placeholder"></div>
		</div>

		{#if isActiveGroup}
			<TabActions />
		{/if}
	</div>
{/if}

<style>
	.tabs-container {
		position: relative;
		height: 38px;
		display: flex;
		align-items: stretch;

		gap: var(--space-xs);
		/* Scope the seam line's z-index so it stays behind the tabs only */
		isolation: isolate;
	}

	.tabs-container::after {
		content: '';
		position: absolute;
		/* content-box: the border-top sits *above* the declared height, which is
		   what lands the seam line flush on the content edge. */
		box-sizing: content-box;
		left: 0;
		right: 0;
		bottom: calc(-1 * var(--br-sm));
		height: var(--br-sm);
		border: var(--border);
		border-bottom: none;
		border-top-left-radius: var(--br-sm);
		border-top-right-radius: var(--br-sm);
		z-index: -1;
		pointer-events: none;
	}

	.tab-nav-arrows {
		display: flex;
		align-items: stretch;
		z-index: 10;
		gap: var(--space-xs-sm);
		padding-bottom: var(--space-xs-sm);
	}
	.wrapper {
		flex: 1;
		display: flex;
		align-items: stretch;
		overflow-x: auto;
		gap: var(--space-xs-sm);
	}
	.placeholder {
		flex: 1;
	}
	.tab-container {
		display: flex;
		align-items: stretch;
	}
	.wrapper .tab {
		position: relative;
		padding: var(--space-xs-sm) var(--space-sm-md) var(--space-xs-sm) var(--space-sm-md);
		display: flex;
		align-items: center;
		border-radius: var(--br-sm);
		background-color: var(--gray-0);
		border-left: var(--bw) transparent solid;
		border-right: var(--bw) transparent solid;
		border-top: var(--bw) transparent solid;

		margin-bottom: var(--space-xs-sm);
		transition:
			background-color 0.05s ease-out 0.03s,
			opacity 0.15s ease-out,
			border-color 0.15s ease-out;
	}
	.wrapper .new-tab {
		padding: 0 var(--space-sm);
		justify-content: center;
	}
	.wrapper .new-tab {
		background: transparent;
	}
	.wrapper .new-tab:hover :global(svg) {
		background: inherit;
		stroke: var(--gray-1000);
	}
	.wrapper .tab .name {
		color: var(--gray-800);
		padding: 0 var(--space-xs) 0 var(--space-xs);
		margin-top: -1px;
	}
	.wrapper .tab:hover .name,
	.wrapper .tab.active .name {
		color: var(--gray-1000);
	}
	.wrapper .tab:hover,
	.wrapper .tab.active {
		background-color: var(--gray-0);
	}
	.wrapper .tab.active {
		/* Concave flare radius. A touch larger than --br-sm since an inverted
		   corner reads tighter than a convex one of the same radius. */
		--flare-r: calc(var(--br-sm) + 2px);
		border-color: var(--border-color);
		border-bottom-color: transparent;
		border-bottom-left-radius: 0;
		border-bottom-right-radius: 0;
		margin-bottom: 0;
	}
	.flare {
		position: absolute;
		bottom: 0;
		width: var(--flare-r);
		height: var(--flare-r);
		overflow: hidden;
		pointer-events: none;
	}
	.flare-left {
		left: calc(-1 * var(--flare-r));
	}
	.flare-right {
		right: calc(-1 * var(--flare-r));
	}
	.flare::after {
		content: '';
		position: absolute;
		box-sizing: border-box;
		width: calc(2 * var(--flare-r));
		height: calc(2 * var(--flare-r));
		top: calc(-1 * var(--flare-r));
		border-radius: 50%;
		border: var(--bw) solid var(--border-color);
		box-shadow: 0 0 0 var(--flare-r) var(--gray-0);
	}
	.flare-left::after {
		left: calc(-1 * var(--flare-r));
	}
	.flare-right::after {
		right: calc(-1 * var(--flare-r));
	}
	:global(.wrapper .tab.active p) {
		color: var(--gray-1000);
		background-color: var(--gray-0);
	}
	.wrapper .tab.dragging {
		opacity: 0.4;
	}
	.drop-indicator {
		width: 0.5px;
		background-color: var(--white-glow);
		box-shadow: 0 0 5px var(--white-glow);
		align-self: stretch;
		flex-shrink: 0;
	}
	/* Close button: pinned to the right of the tab, revealed only on hover. */
	:global(.wrapper .tab .closeBtn) {
		position: absolute;
		right: var(--space-xs-sm);
		top: 50%;
		transform: translateY(-50%);
		z-index: 2;
		opacity: 0;
		transition: opacity 0.15s ease-in-out;
		pointer-events: none;
	}
	:global(.wrapper .tab:hover .closeBtn) {
		opacity: 1;
		pointer-events: auto;
	}
	.wrapper .tab:not(.new-tab)::after {
		content: '';
		position: absolute;
		right: 0;
		top: 0;
		bottom: 0;
		width: 3em;
		border-top-right-radius: var(--br-sm);
		border-bottom-right-radius: var(--br-sm);
		background: linear-gradient(to left, var(--gray-0) 60%, transparent);
		opacity: 0;
		transition: opacity 0.15s ease-in-out;
		pointer-events: none;
		z-index: 1;
	}
	.wrapper .tab:not(.new-tab):hover::after {
		opacity: 1;
	}
</style>
