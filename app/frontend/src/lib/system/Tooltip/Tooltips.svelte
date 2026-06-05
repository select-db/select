<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import FloatingBox from '../FloatingBox/FloatingBox.svelte';
	import { tooltipStore } from './tooltipStore';

	let tooltip = $state($tooltipStore);
	let tooltipElement: HTMLDivElement | null = $state(null);
	let tooltipPosition = $state({ x: 0, y: 0 });

	const unsubscribe = tooltipStore.subscribe((val) => {
		tooltip = val;
	});

	onDestroy(unsubscribe);

	const updateTooltipPosition = async () => {
		if (!tooltip?.anchor || !tooltipElement) return;

		await tick(); // Wait for tooltip to render with content

		const triggerRect = tooltip.anchor.getBoundingClientRect();
		const tooltipRect = tooltipElement.getBoundingClientRect();

		let top = 0;
		let left = 0;

		switch (tooltip.position) {
			case 'top':
				top = triggerRect.top - tooltipRect.height - 8;
				left = triggerRect.left + triggerRect.width / 2 - tooltipRect.width / 2;
				break;
			case 'bottom':
				top = triggerRect.bottom + 8;
				left = triggerRect.left + triggerRect.width / 2 - tooltipRect.width / 2;
				break;
			case 'left':
				top = triggerRect.top + triggerRect.height / 2 - tooltipRect.height / 2;
				left = triggerRect.left - tooltipRect.width - 8;
				break;
			case 'right':
				top = triggerRect.top + triggerRect.height / 2 - tooltipRect.height / 2;
				left = triggerRect.right + 8;
				break;
		}

		tooltipPosition = { x: left, y: top };
	};

	$effect(() => {
		if (tooltip && tooltipElement) {
			updateTooltipPosition();
		}
	});
</script>

{#if tooltip}
	{#key tooltip.id}
		<FloatingBox position={tooltipPosition} appearDelay={tooltip.appearDelay} topLayer>
			<div
				bind:this={tooltipElement}
				class="tooltip {tooltip.actionable ? 'actionable' : ''}"
				class:actionable={tooltip.actionable}
				class:capitalize={tooltip.capitalize}
			>
				<p>{tooltip.text}</p>
			</div>
		</FloatingBox>
	{/key}
{/if}

<style>
	.tooltip {
		background-color: var(--gray-600);
		border-radius: var(--br-xs);
		padding: var(--space-xs) var(--space-sm) 5px var(--space-sm);
		pointer-events: none;
	}

	.tooltip.actionable {
		will-change: transform, opacity;
		backface-visibility: hidden;
		contain: layout style paint;
	}

	.tooltip.capitalize p {
		text-transform: capitalize;
	}
</style>
