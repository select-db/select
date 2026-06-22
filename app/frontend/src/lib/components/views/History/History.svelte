<script lang="ts">
	import { type Component } from 'svelte';
	import { get } from 'svelte/store';

	import Icon from '$lib/system/Icon/Icon.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { scrollShadow } from '$lib/actions/scrollShadow';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { formatRelativeTime } from '$lib/utils/formatRelativeTime';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import type { history } from '$lib/wailsjs/go/models';

	import {
		historyItems,
		historyLoading,
		historyDone,
		historyRefreshNonce,
		resetHistory,
		loadMoreHistory
	} from './historyStore';
	import { buildDbInstanceLookup } from './resolveDbInstance';
	import HistoryStatementModal from './HistoryStatementModal.svelte';

	const dbLookup = $derived(buildDbInstanceLookup($workspaceGraphStore));

	// Reload whenever the workspace changes or a new statement is recorded.
	$effect(() => {
		void $workspaceGraphStore?.id;
		void $historyRefreshNonce;
		void resetHistory();
	});

	function dbNameFor(item: history.HistoryEntry): string {
		return dbLookup.get(item.dbInstanceId)?.name ?? '';
	}

	function hasError(item: history.HistoryEntry): boolean {
		return (item.errors?.length ?? 0) > 0;
	}

	function firstLine(statement: string): string {
		const line = statement.split('\n').find((l) => l.trim().length > 0);
		return (line ?? statement).trim();
	}

	function openStatement(item: history.HistoryEntry): void {
		modalStore.set({
			content: () => HistoryStatementModal as unknown as Component,
			width: 'min(80vw, 900px)',
			height: 'min(70vh, 600px)',
			props: {
				sql: item.statement,
				dbName: dbNameFor(item) || 'unknown database',
				hasError: hasError(item),
				createdAt: item.createdAt
			}
		});
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
		<div class="list no-scrollbar" use:scrollShadow>
			{#each $historyItems as item (item.id)}
				{@const dbName = dbNameFor(item)}
				{@const error = hasError(item)}
				<button class="row" onclick={() => openStatement(item)}>
					<div class="row-top">
						<span class="db">
							<Icon icon="server" size={13} />
							{#if dbName}
								<span class="db-name">{dbName}</span>
							{:else}
								<span class="db-name placeholder">unknown</span>
							{/if}
						</span>
						<span class="row-meta">
							<span class="status" class:error>
								<Icon icon={error ? 'cross' : 'check'} size={12} />
							</span>
							<span class="time">{formatRelativeTime(item.createdAt)}</span>
						</span>
					</div>
					<span class="statement">{firstLine(item.statement)}</span>
				</button>
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
	}

	.row {
		display: flex;
		flex-direction: column;
		gap: var(--space-xxs);
		padding: var(--space-sm) var(--space-sm-md);
		border: none;
		border-bottom: var(--border);
		background: transparent;
		text-align: left;
		cursor: pointer;
		width: 100%;
	}

	.row:hover {
		background-color: var(--gray-200);
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

	.db-name {
		font-size: var(--fs-sm);
		color: var(--gray-1000);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.db-name.placeholder {
		color: var(--gray-700);
		font-style: italic;
	}

	.row-meta {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		flex-shrink: 0;
	}

	.status {
		display: flex;
		align-items: center;
		color: var(--green, var(--gray-800));
	}

	.status.error {
		color: var(--red, var(--orange, var(--gray-800)));
	}

	.time {
		font-size: 11px;
		color: var(--gray-800);
		white-space: nowrap;
	}

	.statement {
		font-family: 'JetBrains Mono', monospace;
		font-size: 11px;
		color: var(--gray-900);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.sentinel {
		display: flex;
		justify-content: center;
		padding: var(--space-sm);
		min-height: 16px;
	}
</style>
