<script lang="ts">
	import File from '$lib/components/views/File/file.svelte';
	import Database from '$lib/components/views/Database/Database.svelte';
	import SchemaTab from '$lib/components/views/Schema/Schema.svelte';
	import DiffView from '$lib/components/views/Diff/DiffView.svelte';
	import Terminal from '$lib/components/views/Terminal/Terminal.svelte';
	import Chat from '$lib/components/views/Chat/Chat.svelte';
	import Settings from '$lib/components/views/Settings/Settings.svelte';
	import QuickActions from '$lib/components/QuickActions/QuickActions.svelte';

	import type { TabGroup } from '../layoutStore';
	import { layoutStore, splitGroup, moveTabToGroup } from '../layoutStore';
	import { dragState } from '../Tabs/tabDragState.svelte';

	import Tabs from '../Tabs/Tabs.svelte';

	type Props = {
		group: TabGroup;
	};

	let { group }: Props = $props();

	type DropZone = 'up' | 'down' | 'left' | 'right' | 'center' | null;
	let hoveredDropZone: DropZone = $state(null);
	let contentElement: HTMLElement | null = $state(null);

	const focusGroup = () => {
		layoutStore.update((l) => ({ ...l, activeGroupId: group.id }));
	};

	// Helper to check if drag event is over the tab bar (not content area)
	const isOverTabBar = (target: HTMLElement) => {
		const wrapper = contentElement?.previousElementSibling as HTMLElement;
		return wrapper?.classList.contains('wrapper') && wrapper.contains(target);
	};

	// Calculate drop zone based on mouse position
	const getDropZone = (e: DragEvent): DropZone => {
		if (!contentElement) return null;

		const rect = contentElement.getBoundingClientRect();
		const x = e.clientX - rect.left;
		const y = e.clientY - rect.top;
		const hThreshold = rect.width * 0.3;
		const vThreshold = rect.height * 0.3;

		if (x < hThreshold) return 'left';
		if (x > rect.width - hThreshold) return 'right';
		if (y < vThreshold) return 'up';
		if (y > rect.height - vThreshold) return 'down';
		return 'center';
	};

	const handleGroupDragOver = (e: DragEvent) => {
		if (!e.dataTransfer?.types.includes('application/x-tab-id') || !contentElement) return;
		if (isOverTabBar(e.target as HTMLElement)) return;

		e.preventDefault();
		hoveredDropZone = getDropZone(e);
	};

	const handleGroupDragLeave = (e: DragEvent) => {
		const relatedTarget = e.relatedTarget as HTMLElement | null;
		if (relatedTarget && contentElement?.contains(relatedTarget)) return;

		hoveredDropZone = null;
	};

	const handleGroupDrop = (e: DragEvent) => {
		if (isOverTabBar(e.target as HTMLElement)) return;

		e.preventDefault();

		const droppedTabId = e.dataTransfer?.getData('application/x-tab-id');
		const sourceGroupId = e.dataTransfer?.getData('application/x-group-id');

		if (droppedTabId && sourceGroupId && hoveredDropZone) {
			if (hoveredDropZone === 'center') {
				moveTabToGroup(droppedTabId, group.id);
			} else {
				splitGroup(group.id, hoveredDropZone, droppedTabId);
			}
		}

		hoveredDropZone = null;
		dragState.tabId = null;
		dragState.groupId = null;
		dragState.hoveredGroupId = null;
	};
</script>

{#if group?.tabs == null}
	<!-- Guard: group can be undefined during layout update -->
{:else}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="group" onpointerdown={focusGroup} onfocusin={focusGroup}>
		<Tabs tabs={group.tabs} groupId={group.id} activeTabId={group.activeTabId} />

		<div
			class="content"
			bind:this={contentElement}
			ondragover={handleGroupDragOver}
			ondragleave={handleGroupDragLeave}
			ondrop={handleGroupDrop}
		>
			{#if group.activeTabId}
				{@const activeTab = group.tabs.find((t) => t.id === group.activeTabId)}
				{#if activeTab}
					{#if activeTab.file}
						<File tab={activeTab} />
					{:else if activeTab.database}
						<Database tab={activeTab} />
					{:else if activeTab.schema}
						<SchemaTab tab={activeTab} />
					{:else if activeTab.diff}
						<DiffView tab={activeTab} />
					{:else if activeTab.terminal}
						<Terminal tab={activeTab} />
					{:else if activeTab.chat}
						{#key activeTab.id}
							<Chat tab={activeTab} />
						{/key}
					{:else if activeTab.settings}
						<Settings />
					{/if}
				{:else}
					<div class="empty">
						<p>Can't find tab</p>
					</div>
				{/if}
			{:else}
				<div class="empty">
					<div class="logo-row">
						<svg class="logo-text" viewBox="0 -880 4487 1024" fill="currentColor"
							><g transform="scale(1, -1)"
								><path
									transform="translate(0, 0)"
									d="M710 525Q710 497 690 478Q671 458 643 458H243Q224 457 211.5 449.5Q199 442 193.0 432.5Q187 423 187 413Q187 403 193.0 393.5Q199 384 211.5 376.5Q224 369 243 368H527Q597 366 642.5 335.5Q688 305 707.5 265.5Q727 226 727 185Q727 145 707.0 105.5Q687 66 642.0 35.5Q597 5 527 5H521Q515 4 509 4H131Q103 4 83.5 23.5Q64 43 64 71Q64 99 84 118Q103 138 131 138H533Q552 137 564.5 145.0Q577 153 583.0 163.5Q589 174 589 185Q589 192 583.0 204.5Q577 217 564.5 225.0Q552 233 533 234H249Q179 236 133.5 266.0Q88 296 68.5 334.5Q49 373 49 413Q49 452 69.0 490.5Q89 529 134.0 559.0Q179 589 249 591H255Q261 592 267 592H643Q671 592 690.5 572.5Q710 553 710 525Z"
								/><path
									transform="translate(776, 0)"
									d="M249 592H295H635Q663 591 681.5 571.0Q700 551 700 523Q695 463 635 458H283Q245 456 220.0 429.0Q195 402 183.5 367.5Q172 333 172 297Q172 261 183.5 226.5Q195 192 220.0 165.0Q245 138 283 136H635Q663 135 681.5 115.0Q700 95 700 67Q699 40 680.5 21.5Q662 3 635 2H268H249Q179 6 133.5 55.5Q88 105 68.5 168.0Q49 231 49 297Q49 363 69.0 426.0Q89 489 134.0 538.5Q179 588 249 592ZM698 301Q698 273 678 254Q659 234 631 234H329Q301 234 281.5 253.5Q262 273 262 301Q262 329 281 349Q301 368 329 368H631Q659 368 678.5 348.5Q698 329 698 301Z"
								/><path
									transform="translate(1513, 0)"
									d="M130 608Q158 608 178.0 589.5Q198 571 199 543V302Q198 261 204.5 226.5Q211 192 236.0 165.0Q261 138 299 136H619Q647 135 665.5 115.0Q684 95 684 67Q683 40 664.5 21.5Q646 3 619 2H265Q195 6 149.5 49.5Q104 93 84 156Q65 219 64 290V543Q65 570 84.0 588.5Q103 607 130 608Z"
								/><path
									transform="translate(2229, 0)"
									d="M249 592H295H635Q663 591 681.5 571.0Q700 551 700 523Q695 463 635 458H283Q245 456 220.0 429.0Q195 402 183.5 367.5Q172 333 172 297Q172 261 183.5 226.5Q195 192 220.0 165.0Q245 138 283 136H635Q663 135 681.5 115.0Q700 95 700 67Q699 40 680.5 21.5Q662 3 635 2H268H249Q179 6 133.5 55.5Q88 105 68.5 168.0Q49 231 49 297Q49 363 69.0 426.0Q89 489 134.0 538.5Q179 588 249 592ZM698 301Q698 273 678 254Q659 234 631 234H329Q301 234 281.5 253.5Q262 273 262 301Q262 329 281 349Q301 368 329 368H631Q659 368 678.5 348.5Q698 329 698 301Z"
								/><path
									transform="translate(2966, 0)"
									d="M291 458Q253 456 227.5 429.0Q202 402 191.0 367.5Q180 333 180 297Q180 261 190.5 226.0Q201 191 227.0 164.5Q253 138 291 136H621Q684 130 690 67Q689 39 669.0 20.5Q649 2 621 2H302H256Q186 6 141.0 55.5Q96 105 76.5 168.0Q57 231 57 297Q57 346 68.0 394.0Q79 442 102.5 485.5Q126 529 165.5 560.0Q205 591 256 592H275H621Q684 586 690 523Q690 495 669.5 476.5Q649 458 621 458Z"
								/><path
									transform="translate(3685, 0)"
									d="M44 525Q44 553 63.5 572.5Q83 592 111 592H693Q721 592 741 573Q760 553 760 525Q760 497 740.5 477.5Q721 458 693 458H470V53Q464 -7 404 -12Q376 -12 355.5 6.5Q335 25 335 53V458H111Q83 458 64 478Q44 497 44 525Z"
								/></g
							></svg
						>
					</div>
					<div class="divider"></div>
					<QuickActions />
				</div>
			{/if}
		</div>

		<!-- Drop zone overlays -->
		{#if dragState.tabId && hoveredDropZone}
			<div class="drop-zone-overlay {hoveredDropZone}"></div>
		{/if}
	</div>
{/if}

<style>
	.group {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
		position: relative;
	}

	.content {
		flex: 1;
		overflow: hidden;
	}

	.empty {
		display: flex;
		flex-direction: column;
		align-items: start;
		height: 65%;
		gap: var(--space-sm);
		padding: var(--space-md) 0;
	}

	.logo-row {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		padding-left: var(--space-md);
		padding-bottom: var(--space-xs);
	}

	.logo-text {
		height: 1.1rem;
		color: var(--gray-800);
	}

	.divider {
		width: 100%;
		border-bottom: var(--border);
	}

	.drop-zone-overlay {
		position: absolute;
		background-color: var(--gray-1000);
		border-radius: var(--br-xs);
		pointer-events: none;
		z-index: 1000;
		opacity: 0.15;
	}

	.drop-zone-overlay.left {
		top: var(--space-xs);
		left: var(--space-xs);
		width: 30%;
		bottom: var(--space-xs);
	}

	.drop-zone-overlay.right {
		top: var(--space-xs);
		right: var(--space-xs);
		width: 30%;
		bottom: var(--space-xs);
	}

	.drop-zone-overlay.up {
		top: var(--space-xs);
		left: var(--space-xs);
		right: var(--space-xs);
		height: 30%;
	}

	.drop-zone-overlay.down {
		bottom: var(--space-xs);
		left: var(--space-xs);
		right: var(--space-xs);
		height: 30%;
	}

	.drop-zone-overlay.center {
		top: var(--space-xs);
		left: var(--space-xs);
		right: var(--space-xs);
		bottom: var(--space-xs);
	}
</style>
