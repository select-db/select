<script lang="ts">
	import { type Component, tick } from 'svelte';
	import { get } from 'svelte/store';

	import Badge from '$lib/system/Badge/Badge.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { scrollShadow } from '$lib/actions/scrollShadow';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { formatRelativeTime } from '$lib/utils/formatRelativeTime';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { findItemById } from '$lib/components/views/FileSystem/Files/helpers/dragHelpers';
	import { colorizeSql } from '$lib/system/SqlViewer/colorizeSql';
	import type { graph, history } from '$lib/wailsjs/go/models';

	import {
		historyItems,
		historyLoading,
		historyDone,
		historyRefreshNonce,
		resetHistory,
		refreshHistory,
		loadMoreHistory
	} from './historyStore';
	import HistoryStatementModal from './HistoryStatementModal.svelte';

	let openedEntryId = $state<string | null>(null);
	let listEl: HTMLElement | undefined = $state();

	// On a real workspace switch, clear and reload (a blank gap is expected). For
	// in-place refreshes (a new statement recorded), swap the list atomically so
	// the panel never flashes to the loader/empty state.
	let lastWorkspaceId: string | undefined;
	$effect(() => {
		const workspaceId = $workspaceGraphStore?.id;
		void $historyRefreshNonce;
		if (workspaceId !== lastWorkspaceId) {
			lastWorkspaceId = workspaceId;
			void resetHistory();
		} else {
			void refreshHistory();
		}
	});

	function dbNameFor(item: history.HistoryEntry): string {
		const ws = $workspaceGraphStore;
		if (!ws) return '';
		const node = findItemById(item.dbInstanceId, [], ws.folders ?? [], ws.db_instances ?? []);
		return node && 'db_type' in node ? (node as graph.DBInstanceNode).name : '';
	}

	function hasError(item: history.HistoryEntry): boolean {
		return (item.errors?.length ?? 0) > 0;
	}

	// eslint-disable-next-line svelte/prefer-svelte-reactivity -- module-level memoization cache; must stay non-reactive
	const previewCache = new Map<string, Promise<string>>();

	function previewHtml(statement: string): Promise<string> {
		let html = previewCache.get(statement);
		if (!html) {
			const lines = statement
				.split('\n')
				.map((l) => l.trim())
				.filter((l) => l.length > 0);
			const snippet = lines
				.slice(0, 4)
				.concat(lines.length > 4 ? ['...'] : [])
				.join(' ');
			html = colorizeSql(snippet);
			previewCache.set(statement, html);
		}
		return html;
	}

	function openStatement(item: history.HistoryEntry): void {
		openedEntryId = item.id;
		const items = get(historyItems);
		const index = items.findIndex((i) => i.id === item.id);
		modalStore.set({
			content: () => HistoryStatementModal as unknown as Component,
			width: 'min(80vw, 900px)',
			height: 'min(70vh, 600px)',
			props: {
				sql: item.statement,
				dbInstanceId: item.dbInstanceId,
				dbName: dbNameFor(item) || 'unknown database',
				hasError: hasError(item),
				errors: item.errors ?? [],
				createdAt: item.createdAt,
				onPrev: () => navigate(-1),
				onNext: () => navigate(1),
				canPrev: index > 0,
				canNext: index >= 0 && index < items.length - 1
			}
		});
		void scrollOpenedIntoView();
	}

	function navigate(delta: number): void {
		const items = get(historyItems);
		const index = items.findIndex((i) => i.id === openedEntryId);
		const next = index === -1 ? undefined : items[index + delta];
		if (next) openStatement(next);
	}

	async function scrollOpenedIntoView(): Promise<void> {
		await tick();
		const list = listEl;
		const row = list?.querySelector(`[data-id="${openedEntryId}"]`);
		if (!list || !row) return;

		const listRect = list.getBoundingClientRect();
		const rowRect = row.getBoundingClientRect();
		const offset = rowRect.top + rowRect.height / 2 - (listRect.top + listRect.height / 2);
		list.scrollBy({ top: offset, behavior: 'smooth' });
	}

	// Infinite scroll: load the next page when the sentinel scrolls into view.
	function observeSentinel(node: HTMLElement) {
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0]?.isIntersecting && !get(historyLoading) && !get(historyDone)) {
					void loadMoreHistory();
				}
			},
			{ threshold: 0.1 }
		);
		observer.observe(node);
		return { destroy: () => observer.disconnect() };
	}
</script>

<div class="history-panel">
	{#if $historyItems.length === 0 && $historyLoading}
		<div class="state">
			<Loader />
		</div>
	{:else if $historyItems.length === 0}
		<div class="state">
			<p class="muted">No statements run in the last 7 days.</p>
		</div>
	{:else}
		<div class="list no-scrollbar" bind:this={listEl} use:scrollShadow={{ top: true }}>
			{#each $historyItems as item (item.id)}
				{@const dbName = dbNameFor(item)}
				{@const error = hasError(item)}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class="row"
					data-id={item.id}
					class:selected={item.id === openedEntryId}
					onclick={() => openStatement(item)}
				>
					<div class="row-top">
						<div class="row-meta">
							<p class="time">{formatRelativeTime(item.createdAt)}</p>
							<Badge status={error ? 'error' : 'success'} />
							{#if dbName}
								<p class="db-name truncate">{dbName}</p>
							{:else}
								<p class="db-name placeholder truncate">unknown</p>
							{/if}
							<div class="db"></div>
						</div>
					</div>
					{#await previewHtml(item.statement) then html}
						<!-- eslint-disable-next-line svelte/no-at-html-tags trusted SQL, escaped by Monaco -->
						<span class="statement truncate">{@html html}</span>
					{/await}
				</div>
			{/each}

			{#if !$historyDone}
				<div class="sentinel" use:observeSentinel>
					{#if $historyLoading}<Loader />{/if}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.history-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.state {
		display: flex;
		justify-content: center;
		padding: var(--space-md);
	}

	.muted {
		color: var(--gray-800);
		font-size: var(--fs-sm);
		text-align: center;
	}

	.list {
		display: flex;
		flex-direction: column;
		overflow-x: hidden;
		overflow-y: auto;
		overscroll-behavior-y: none;
		flex-grow: 1;

		padding: 0 var(--space-sm);
	}

	.row {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding: var(--space-sm) var(--space-sm);
		border: var(--bw) transparent solid;
		border-radius: var(--br-sm);
		background: transparent;
		text-align: left;
		overflow: hidden;
		min-height: 39px;

		transition: all 0.2s ease;
	}

	.row:hover {
		border: var(--border);
	}

	.row p {
		color: var(--gray-800);
		transition: all 0.1s ease;
	}

	.row:hover p {
		color: var(--gray-1000);
	}

	.row.selected {
		border: var(--border);
		background-color: var(--gray-200);
	}

	.row.selected p {
		color: var(--gray-1000);
	}

	.row-top {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-sm);
	}

	.db {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		min-width: 0;
	}

	.db-name.placeholder {
		color: var(--gray-700);
		font-style: italic;
	}

	.row-meta {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		flex-shrink: 0;
	}

	.time {
		color: var(--gray-800);
		white-space: nowrap;
	}

	.statement {
		font-family: 'JetBrains Mono', monospace;
		font-size: 11px;
	}

	.sentinel {
		display: flex;
		justify-content: center;
		padding: var(--space-sm);
		min-height: 16px;
	}
</style>
