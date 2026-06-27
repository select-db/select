<script lang="ts">
	import Badge from '$lib/system/Badge/Badge.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import DatabaseIndicator from '$lib/components/shared/DatabaseIndicator/DatabaseIndicator.svelte';
	import SqlViewer from '$lib/system/SqlViewer/SqlViewer.svelte';
	import { formatRelativeTime } from '$lib/utils/formatRelativeTime';

	type Props = {
		sql: string;
		dbInstanceId: string;
		dbName: string;
		hasError: boolean;
		errors: string[];
		createdAt: string;
		onPrev: () => void;
		onNext: () => void;
		canPrev: boolean;
		canNext: boolean;
		onClose: () => void;
	};

	let {
		sql,
		dbInstanceId,
		dbName,
		hasError,
		errors,
		createdAt,
		onPrev,
		onNext,
		canPrev,
		canNext
	}: Props = $props();
</script>

<div class="wrapper">
	<div class="header">
		<span class="meta">
			<DatabaseIndicator size={16} id={dbInstanceId} />
			<span class="title">{dbName}</span>
		</span>
		<span class="header-right">
			<p class="hint">{formatRelativeTime(createdAt)}</p>
			<Badge status={hasError ? 'error' : 'success'} label />
			<span class="nav">
				<Button
					leftIcon="chevron-up"
					iconSize={16}
					size="sm"
					disabled={!canPrev}
					onclick={onPrev}
					emphasis="high"
				/>
				<Button
					leftIcon="chevron-down"
					iconSize={16}
					size="sm"
					disabled={!canNext}
					onclick={onNext}
					emphasis="high"
				/>
			</span>
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
	.wrapper {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
		background-color: var(--gray-0);
	}

	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-xs-sm);
		border-bottom: var(--border);
		background-color: var(--gray-400);
	}

	.errors {
		border-bottom: var(--border);
		padding: var(--space-sm-md) var(--space-sm);
	}

	.meta {
		display: flex;
		gap: var(--space-xs-sm);
		padding-left: var(--space-xs-sm);
		align-items: center;
		min-width: 0;
	}

	.title {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.header-right {
		display: flex;
		gap: var(--space-sm);
		align-items: center;
		flex-shrink: 0;
	}

	.nav {
		display: flex;
		align-items: center;
		gap: var(--space-xs-sm);
	}

	.hint {
		font-size: 11px;
		color: var(--gray-800);
	}

	.editor {
		flex: 1;
		min-height: 0;
		width: 100%;
	}
</style>
