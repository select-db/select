<script lang="ts">
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Tooltip from '$lib/system/Tooltip/Tooltip.svelte';
	import type { Icons } from '$lib/system/Icon/types';

	type Option = {
		id: string;
		icon?: Icons;
		label?: string;
		tooltip?: string;
	};

	let {
		options = [],
		value = null,
		onSelect = () => {}
	}: {
		options?: Option[];
		value?: Option['id'] | null;
		onSelect?: (value: Option['id']) => void;
	} = $props();

	const handleSelect = (e: MouseEvent | KeyboardEvent, value: Option['id']) => {
		e.preventDefault();
		e.stopPropagation();
		onSelect(value);
	};

	let segmentRefs: (HTMLDivElement | undefined)[] = [];
	let containerRef: HTMLDivElement | null = null;
	let segmentWidths = $state<number[]>([]);
	let observers: ResizeObserver[] = [];

	const activeIndex = $derived(options.findIndex((o) => o.id === value));

	const indicatorLeft = $derived.by(() => {
		if (activeIndex === -1) return 0;
		return segmentWidths.slice(0, activeIndex).reduce((sum, width) => sum + (width ?? 0), 0);
	});

	const indicatorWidth = $derived.by(() => {
		if (activeIndex === -1) return 0;
		const width = segmentWidths[activeIndex];
		if (width && width > 0) return width;
		// Fallback: calculate from container or use minimum
		if (containerRef && segmentRefs.length > 0) {
			return containerRef.offsetWidth / segmentRefs.length;
		}
		return 50;
	});

	const updateSegmentWidths = () => {
		segmentWidths = segmentRefs.map((ref) => ref?.offsetWidth ?? 0);
	};

	$effect(() => {
		if (segmentRefs.length !== options.length) {
			segmentRefs = new Array(options.length);
		}
	});

	$effect(() => {
		observers.forEach((observer) => observer.disconnect());
		observers = [];

		const setupObservers = () => {
			const allRefsReady = segmentRefs.length > 0 && segmentRefs.every((ref) => ref !== undefined);
			if (!allRefsReady) return false;

			segmentRefs.forEach((ref) => {
				if (!ref) return;
				const observer = new ResizeObserver(updateSegmentWidths);
				observer.observe(ref);
				observers.push(observer);
			});

			updateSegmentWidths();
			return true;
		};

		let timeoutId: ReturnType<typeof setTimeout> | null = null;
		if (!setupObservers()) {
			timeoutId = setTimeout(setupObservers, 0);
		}

		return () => {
			if (timeoutId) clearTimeout(timeoutId);
			observers.forEach((observer) => observer.disconnect());
			observers = [];
		};
	});

	$effect(() => {
		const rafId = requestAnimationFrame(updateSegmentWidths);
		return () => cancelAnimationFrame(rafId);
	});
</script>

{#snippet segment(option: Option, index: number)}
	<div
		bind:this={segmentRefs[index]}
		class="segment {option.id === value ? 'active' : 'inactive'}"
		role="radio"
		aria-checked={option.id === value}
		tabindex="0"
		onclick={(e) => handleSelect(e, option.id)}
		onkeydown={(e) => {
			if (e.key === 'Enter' || e.key === ' ') {
				handleSelect(e, option.id);
			}
		}}
	>
		{#if option.icon}
			<Icon
				icon={option.icon}
				stroke={option.id === value ? 'var(--gray-1000)' : 'var(--gray-700)'}
				size={16}
			/>
		{/if}
		{#if option.label}
			<p>{option.label}</p>
		{/if}
	</div>
{/snippet}

<div class="segmented-control" bind:this={containerRef} role="radiogroup">
	{#each options as option, index (option.id)}
		{#if option.tooltip}
			<Tooltip text={option.tooltip} capitalize>
				{@render segment(option, index)}
			</Tooltip>
		{:else}
			{@render segment(option, index)}
		{/if}
	{/each}
	<div class="active-indicator" style="left: {indicatorLeft}px; width: {indicatorWidth}px;"></div>
</div>

<style>
	.segmented-control {
		position: relative;
		display: inline-flex;
	}

	.segment {
		position: relative;
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		padding: var(--space-xs-sm) var(--space-sm);
		color: var(--gray-1000);
		z-index: 1;
		transition: all 0.2s;
	}

	:global(.segment svg) {
		transition: stroke 0.2s;
	}
	:global(.segment.active svg) {
		stroke: var(--gray-1000);
	}
	:global(.segment.inactive svg) {
		stroke: var(--gray-800);
	}
	:global(.segment.inactive:hover svg) {
		stroke: var(--gray-1000);
	}

	.segment p {
		transition: color 0.2s;
	}
	.segment.active p {
		color: var(--gray-1000);
	}
	.segment.inactive p {
		color: var(--gray-800);
	}
	.segment.inactive:hover p {
		color: var(--gray-1000);
	}

	.active-indicator {
		position: absolute;
		top: 0;
		bottom: 0;
		background-color: var(--gray-0);

		border: var(--border);
		border-radius: var(--br-xs);

		transition:
			left 0.15s ease,
			width 0.15s ease;
		z-index: 0;
	}
</style>
