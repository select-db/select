<script lang="ts">
	import { tooltipStore } from './tooltipStore';

	type TooltipProps = {
		text: string;
		position?: 'top' | 'bottom' | 'left' | 'right';
		appearDelay?: number;
		actionable?: boolean;
		capitalize?: boolean;
		children?: import('svelte').Snippet;
	};

	let {
		text,
		position = 'top',
		appearDelay = 800,
		actionable = true,
		capitalize,
		children
	}: TooltipProps = $props();

	let triggerElement: HTMLElement | null = $state(null);
	let tooltipId = `tooltip-${Math.random().toString(36).substr(2, 9)}`;
	let isHovering = $state(false);

	const handleMouseEnter = () => {
		if (!triggerElement) return;
		isHovering = true;
		tooltipStore.show({
			id: tooltipId,
			text,
			anchor: triggerElement,
			position,
			actionable,
			appearDelay,
			capitalize
		});
	};

	const handleMouseLeave = () => {
		isHovering = false;
		tooltipStore.hide();
	};

	// Update tooltip text reactively if it changes while hovering
	$effect(() => {
		if (isHovering && triggerElement) {
			tooltipStore.show({
				id: tooltipId,
				text,
				anchor: triggerElement,
				position,
				actionable,
				appearDelay,
				capitalize
			});
		}
	});

	// Hide tooltip when component is destroyed
	$effect(() => {
		return () => {
			if (isHovering) {
				tooltipStore.hide();
			}
		};
	});
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="tooltip-container"
	bind:this={triggerElement}
	onmouseenter={handleMouseEnter}
	onmouseleave={handleMouseLeave}
>
	{@render children?.()}
</div>

<style>
	.tooltip-container {
		display: inline-block;
	}
</style>
