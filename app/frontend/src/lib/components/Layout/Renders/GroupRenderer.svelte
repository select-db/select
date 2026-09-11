<script lang="ts">
	/**
	 * The views, loaded when a tab of that kind is opened rather than with the
	 * app. This component is reached from the root layout, so importing them here
	 * meant every one of them -- and the terminal's xterm, the chat's katex and
	 * marked, the table's d3 -- was parsed before the window could paint, whether
	 * or not anything was open.
	 */
	const File = () => import('$lib/components/views/File/file.svelte');
	const Database = () => import('$lib/components/views/Database/Database.svelte');
	const SchemaTab = () => import('$lib/components/views/Schema/Schema.svelte');
	const DiffView = () => import('$lib/components/views/Diff/DiffView.svelte');
	const Terminal = () => import('$lib/components/views/Terminal/Terminal.svelte');
	const Chat = () => import('$lib/components/views/Chat/Chat.svelte');
	const Settings = () => import('$lib/components/views/Settings/Settings.svelte');
	import QuickActions from '$lib/components/QuickActions/QuickActions.svelte';
	import Wordmark from '$lib/components/shared/Wordmark/Wordmark.svelte';

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
			data-test="group.content"
			bind:this={contentElement}
			ondragover={handleGroupDragOver}
			ondragleave={handleGroupDragLeave}
			ondrop={handleGroupDrop}
		>
			{#if group.activeTabId}
				{@const activeTab = group.tabs.find((t) => t.id === group.activeTabId)}
				{#if activeTab}
					{#if activeTab.file}
						{#await File() then { default: View }}<View tab={activeTab} />{/await}
					{:else if activeTab.database}
						{#await Database() then { default: View }}<View tab={activeTab} />{/await}
					{:else if activeTab.schema}
						{#await SchemaTab() then { default: View }}<View tab={activeTab} />{/await}
					{:else if activeTab.diff}
						{#await DiffView() then { default: View }}<View tab={activeTab} />{/await}
					{:else if activeTab.terminal}
						{#await Terminal() then { default: View }}<View tab={activeTab} />{/await}
					{:else if activeTab.chat}
						{#key activeTab.id}
							{#await Chat() then { default: View }}<View tab={activeTab} />{/await}
						{/key}
					{:else if activeTab.settings}
						{#await Settings() then { default: View }}<View />{/await}
					{/if}
				{:else}
					<div class="empty">
						<p>Can't find tab</p>
					</div>
				{/if}
			{:else}
				<div class="empty">
					<div class="logo-row">
						<Wordmark height="1.1rem" color="var(--gray-800)" />
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
		border-left: var(--border);
		border-right: var(--border);
		border-bottom: var(--border);
		border-radius: var(--br-sm);
		background-color: var(--gray-200);
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
