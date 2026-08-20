<script lang="ts">
	import { onDestroy } from 'svelte';
	import FloatingBox from '../FloatingBox/FloatingBox.svelte';
	import { tooltipStore } from './tooltipStore';

	let tooltip = $state($tooltipStore);

	const unsubscribe = tooltipStore.subscribe((val) => {
		tooltip = val;
	});

	onDestroy(unsubscribe);
</script>

{#if tooltip}
	{#key tooltip.id}
		<!-- Placement (including flipping to the opposite side when the preferred one
		     is too tight) is FloatingBox's job: it is the one that knows the box's
		     size, the zoom and the screen margins. passthrough keeps the tooltip from
		     ever swallowing a click meant for the element it describes. -->
		<FloatingBox
			anchor={tooltip.anchor}
			placement={tooltip.position}
			offset={{ x: 8, y: 8 }}
			appearDelay={tooltip.appearDelay}
			topLayer
			passthrough
		>
			<div
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
		background-color: var(--gray-300);
		border-radius: var(--br-xs);
		border: var(--border-contrast);
		padding: var(--space-sm);
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
