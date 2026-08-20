<script lang="ts">
	import { onMount, tick, onDestroy } from 'svelte';
	import { fade, scale } from 'svelte/transition';
	import { lastHideTime, setLastHideTime } from '$lib/system/Tooltip/tooltipStore';

	const SCREEN_MARGIN = 8;
	const QUICK_SWITCH_THRESHOLD = 1500; // Time in ms to consider it a quick switch

	let {
		anchor,
		position = { x: 0, y: 0 },
		offset = { x: 0, y: 0 },
		backdrop = false,
		onBackdropClick,
		appearDelay = 0,
		topLayer = false,
		children
	}: {
		anchor?: HTMLElement | null;
		position?: { x: number; y: number };
		offset?: { x: number; y: number };
		backdrop?: boolean;
		onBackdropClick?: () => void;
		appearDelay?: number;
		/** When true, uses a higher z-index so the box renders above menus and other overlays. Use for tooltips. */
		topLayer?: boolean;
		children: import('svelte').Snippet;
	} = $props();

	let boxElement = $state<HTMLDivElement | undefined>(undefined);
	let boxPosition = $state({ x: 0, y: 0 });
	let maxWidth = $state<number | null>(null);
	let visible = $state(false);
	let timeoutId: number | null = null;

	/**
	 * The app scales the whole UI with `zoom` on <html> (see zoomStore). Client
	 * coordinates — mouse events, getBoundingClientRect() — come back multiplied
	 * by it, while the left/top we write out are multiplied again on render. So
	 * divide incoming client values by the zoom and do the math in the same
	 * (zoomed) space the box is laid out in.
	 */
	const rootZoom = () => {
		const zoom = parseFloat(getComputedStyle(document.documentElement).zoom);
		return Number.isFinite(zoom) && zoom > 0 ? zoom : 1;
	};

	const calculatePosition = async () => {
		if (!boxElement) return;

		await tick(); // Wait for content to render
		if (!boxElement) return; // destroyed while awaiting

		const zoom = rootZoom();
		const viewport = {
			width: window.innerWidth / zoom,
			height: window.innerHeight / zoom
		};

		// Never let the box itself be wider than the safe area: `width: fit-content`
		// otherwise measures a box that can't be nudged back inside the screen.
		maxWidth = Math.max(0, viewport.width - SCREEN_MARGIN * 2);

		// offsetWidth/offsetHeight, not getBoundingClientRect(): the box measures
		// itself while its `scale` intro transition is running, and the rect would
		// report the mid-animation size (~10% short) — which is exactly how much of
		// the right/bottom margin went missing.
		const box = { width: boxElement.offsetWidth, height: boxElement.offsetHeight };

		let x = position.x / zoom + offset.x;
		let y = position.y / zoom + offset.y;

		// If anchor element is provided, position relative to it
		const anchorRect = anchor?.getBoundingClientRect();
		if (anchorRect) {
			x = anchorRect.left / zoom;
			y = anchorRect.bottom / zoom + offset.y;
		}

		// Adjust horizontal position to stay on screen with margin
		if (x + box.width > viewport.width - SCREEN_MARGIN) {
			x = viewport.width - box.width - SCREEN_MARGIN;
		}
		if (x < SCREEN_MARGIN) x = SCREEN_MARGIN;

		// Adjust vertical position to stay on screen with margin
		if (y + box.height > viewport.height - SCREEN_MARGIN) {
			if (anchorRect) {
				// Try positioning above the anchor
				const aboveY = anchorRect.top / zoom - box.height - offset.y;
				if (aboveY >= SCREEN_MARGIN) {
					y = aboveY;
				} else {
					// If it doesn't fit above either, position at bottom with margin
					y = viewport.height - box.height - SCREEN_MARGIN;
				}
			} else {
				y = viewport.height - box.height - SCREEN_MARGIN;
			}
		}
		if (y < SCREEN_MARGIN) y = SCREEN_MARGIN;

		// Only write when it actually moved: the resize observer re-measures often
		// and an identical object would still churn a re-render.
		if (boxPosition.x !== x || boxPosition.y !== y) boxPosition = { x, y };
	};

	onMount(() => {
		calculatePosition();

		// Check if this is a quick switch from another tooltip
		const now = Date.now();
		const timeSinceLastHide = now - lastHideTime;
		const shouldDelay = appearDelay > 0 && timeSinceLastHide > QUICK_SWITCH_THRESHOLD;

		if (shouldDelay) {
			timeoutId = window.setTimeout(() => (visible = true), appearDelay);
		} else {
			visible = true;
		}
	});

	onDestroy(() => {
		if (timeoutId !== null) {
			clearTimeout(timeoutId);
		}
		setLastHideTime(Date.now());
	});

	// Recalculate on window resize
	$effect(() => {
		const handleResize = () => calculatePosition();
		window.addEventListener('resize', handleResize);
		return () => window.removeEventListener('resize', handleResize);
	});

	// ...and whenever the content itself resizes (async menu options, a filtered
	// list, a growing tooltip): the first measure alone would leave a grown box
	// hanging past the screen margin.
	$effect(() => {
		const el = boxElement;
		if (!el) return;
		const observer = new ResizeObserver(() => calculatePosition());
		observer.observe(el);
		return () => observer.disconnect();
	});

	// Recalculate when position or anchor changes
	$effect(() => {
		void position;
		void anchor;
		calculatePosition();
	});
</script>

{#if visible}
	{#if backdrop}
		<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="backdrop"
			class:top-layer={topLayer}
			onmousedown={onBackdropClick}
			tabindex="0"
			oncontextmenu={(e) => (e.stopPropagation(), e.preventDefault())}
			transition:fade={{ duration: 100 }}
		></div>
	{/if}

	<div
		bind:this={boxElement}
		class="floating-box"
		class:top-layer={topLayer}
		style="left: {boxPosition.x}px; top: {boxPosition.y}px;{maxWidth === null
			? ''
			: ` max-width: ${maxWidth}px;`}"
		transition:scale={{ duration: 100, start: 0.9 }}
	>
		{@render children()}
	</div>
{/if}

<style>
	.backdrop {
		position: fixed;
		/* inset, not 100vw/100vh: viewport units ignore the root `zoom` and would
		   overflow the window once the UI is zoomed in. */
		inset: 0;
		z-index: 999;
	}

	.backdrop.top-layer {
		z-index: 9999;
	}

	:global([data-theme='light']) .backdrop {
		background: hsla(var(--hue), 12%, 94%, 0.2);
	}

	:global([data-theme='dark']) .backdrop {
		background: hsla(var(--hue), 12%, 12%, 0.3);
	}

	.floating-box {
		position: fixed;
		z-index: 1000;
		overflow: hidden;
		box-shadow: var(--shadow) 0px 7px 29px 0px !important;
		width: fit-content;
	}

	.floating-box.top-layer {
		z-index: 10000;
	}
</style>
