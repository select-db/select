<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import SqlViewer from '$lib/system/SqlViewer/SqlViewer.svelte';
	import { formatRelativeTime } from '$lib/utils/formatRelativeTime';

	type Props = {
		sql: string;
		dbName: string;
		hasError: boolean;
		createdAt: string;
		onClose: () => void;
	};

	let { sql, dbName, hasError, createdAt }: Props = $props();
</script>

<ModalHeader title="Statement" icon="clock" />

<div class="history-modal-body">
	<div class="toolbar">
		<span class="meta">
			<Icon icon="server" size={14} />
			<span class="db-name">{dbName}</span>
		</span>
		<span class="toolbar-right">
			<span class="status" class:error={hasError}>
				<Icon icon={hasError ? 'cross' : 'check'} size={12} />
				{hasError ? 'Error' : 'Success'}
			</span>
			<span class="time">{formatRelativeTime(createdAt)}</span>
		</span>
	</div>

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

	.status {
		display: flex;
		gap: var(--space-xxs);
		align-items: center;
		font-size: 11px;
		color: var(--green, var(--gray-800));
	}

	.status.error {
		color: var(--red, var(--orange, var(--gray-800)));
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
