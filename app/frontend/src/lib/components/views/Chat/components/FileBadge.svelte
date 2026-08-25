<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import ItemIcon from '$lib/components/views/shared/ItemIcon.svelte';
	import type * as graph from '$lib/wails/graph';

	type Props = {
		node: graph.FileNode;
		onRemove: () => void;
	};

	let { node, onRemove }: Props = $props();
</script>

<div class="file-badge">
	<ItemIcon item={node} noDepth />
	<span class="file-name">{node.name}</span>
	<Button
		emphasis="low"
		size="sm"
		leftIcon="cross"
		iconSize={13}
		onclick={onRemove}
		ariaLabel="Remove file"
		classes="remove-btn"
	/>
</div>

<style>
	.file-badge {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		padding: var(--space-xxs) var(--space-xs) var(--space-xxs) var(--space-xs-sm);
		background: transparent;
		border: 1px solid transparent;
		border-radius: var(--br-sm);
		font-size: var(--fs-sm);
		transition: all 0.15s;
	}

	.file-badge:hover {
		background: var(--gray-100);
		border-color: var(--border-color);
	}

	.file-badge .file-name {
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--gray-800);
		transition: color 0.15s;
	}

	.file-badge:hover .file-name {
		color: var(--gray-1000);
	}

	.file-badge :global(.remove-btn) {
		opacity: 0;
		transition: opacity 0.15s;
	}

	.file-badge:hover :global(.remove-btn) {
		opacity: 1;
	}
</style>
