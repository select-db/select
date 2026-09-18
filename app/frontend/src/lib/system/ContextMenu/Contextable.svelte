<script lang="ts">
	import type { Snippet } from 'svelte';

	import ContextMenu from './ContextMenu.svelte';
	import type { ContextMenuOption } from './types';

	let {
		options,
		direction,
		metadata,
		disabled = false,
		style,
		anchor = 'cursor',
		on = 'context',
		menuWidth,
		children
	}: {
		options: ContextMenuOption[] | (() => ContextMenuOption[]);
		direction: 'right' | 'left';
		metadata?: unknown;
		disabled?: boolean;
		style?: string;
		anchor?: 'cursor' | 'child';
		on?: 'click' | 'context';
		menuWidth?: number;
		children?: Snippet;
	} = $props();

	let isOpen = $state(false);
	let position = $state({ x: 0, y: 0 });
	let containerRef: HTMLDivElement | null = null;
	let resolvedOptions = $state<ContextMenuOption[]>([]);

	const metadataId = $derived(() => {
		if (!metadata || typeof metadata !== 'object') return '';
		if (!('id' in metadata)) return '';
		const id = metadata.id;
		return typeof id === 'string' ? id : '';
	});

	const handleClick = (e: MouseEvent) => {
		e.preventDefault();
		e.stopPropagation();

		const resolved = typeof options === 'function' ? options() : options;
		if (resolved.length === 0) return;
		if (disabled) return;

		resolvedOptions = resolved;

		isOpen = !isOpen;

		if (!isOpen) return;
		if (!containerRef) return;

		const rect = containerRef.getBoundingClientRect();

		switch (anchor) {
			case 'child':
				position = { x: rect.left, y: rect.bottom };
				break;
			default:
				position = { x: e.clientX, y: e.clientY };
				break;
		}
	};

	const onClose = () => (isOpen = false);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	{style}
	bind:this={containerRef}
	oncontextmenu={(e: MouseEvent) => {
		if (!(on === 'context')) return;
		handleClick(e);
	}}
	onclick={(e: MouseEvent) => {
		if (!(on === 'click')) return;
		handleClick(e);
	}}
	data-id={metadataId}
	class="no-scrollbar"
>
	{@render children?.()}

	{#if isOpen}
		<ContextMenu
			{position}
			options={resolvedOptions}
			{direction}
			{metadata}
			{anchor}
			{onClose}
			{menuWidth}
		/>
	{/if}
</div>
