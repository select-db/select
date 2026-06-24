<script lang="ts">
	import Badge from '$lib/system/Badge/Badge.svelte';
	import SqlViewer from '$lib/system/SqlViewer/SqlViewer.svelte';
	import { formatRelativeTime } from '$lib/utils/formatRelativeTime';

	type Props = {
		sql: string;
		dbName: string;
		hasError: boolean;
		errors: string[];
		createdAt: string;
		onClose: () => void;
	};

	let { sql, dbName, hasError, errors, createdAt }: Props = $props();
</script>

<div class="history-modal-body">
	<div class="toolbar">
		<span class="meta">
			<span class="db-name">{dbName}</span>
		</span>
		<span class="toolbar-right">
			<Badge status={hasError ? 'error' : 'success'} label />
			<p class="time">{formatRelativeTime(createdAt)}</p>
		</span>
	</div>

	{#if errors.length > 0}
		<div class="errors">
			<Badge status={hasError ? 'error' : 'success'} label={errors.join('\n')} />
		</div>
	{/if}

	<div class="editor">
		<SqlViewer {sql} />
	</div>
</div>

<style>
	.history-modal-body {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
		background-color: var(--gray-0);
	}

	.toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-sm);
		border-bottom: var(--border);
	}

	.errors {
		border-bottom: var(--border);
		padding: var(--space-sm-md) var(--space-sm);
	}

	.meta {
		display: flex;
		gap: var(--space-xs);
		align-items: center;
		min-width: 0;
	}

	.db-name {
		font-size: var(--fs-sm);
		color: var(--gray-1000);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.toolbar-right {
		display: flex;
		gap: var(--space-sm);
		align-items: center;
		flex-shrink: 0;
	}

	.time {
		font-size: 11px;
		color: var(--gray-800);
	}

	.editor {
		flex: 1;
		min-height: 0;
		width: 100%;
	}
</style>
