<script lang="ts">
	import { onMount, tick, onDestroy } from 'svelte';
	import { fade, scale } from 'svelte/transition';
	import { lastHideTime, setLastHideTime } from '$lib/system/Tooltip/tooltipStore';

	const SCREEN_MARGIN = 8;
	const QUICK_SWITCH_THRESHOLD = 1500; // Time in ms to consider it a quick switch

	let {
		anchor,
		placement = 'bottom-start',
		position = { x: 0, y: 0 },
		offset = { x: 0, y: 0 },
		backdrop = false,
		onBackdropClick,
		appearDelay = 0,
		topLayer = false,
		passthrough = false,
		children
	}: {
		anchor?: HTMLElement | null;
		/**
		 * Where the box sits relative to `anchor`. 'bottom-start' hangs it under the
		 * anchor's left edge (menus, selects). The four side placements center the
		 * box on that side of the anchor and flip to the opposite side when it
		 * doesn't fit, so the box never covers the element it describes. Ignored
		 * when there is no anchor.
		 */
		placement?: Placement;
		position?: { x: number; y: number };
		/** Gap from the anchor on the main axis for side placements; a plain offset otherwise. */
		offset?: { x: number; y: number };
		backdrop?: boolean;
		onBackdropClick?: () => void;
		appearDelay?: number;
		/** When true, uses a higher z-index so the box renders above menus and other overlays. Use for tooltips. */
		topLayer?: boolean;
		/** Click-through: the box takes no pointer events, so it can never swallow a click meant for what's underneath. */
		passthrough?: boolean;
		children: import('svelte').Snippet;
	} = $props();

	let boxElement = $state<HTMLDivElement | undefined>(undefined);
	let boxPosition = $state({ x: 0, y: 0 });
	let maxWidth = $state<number | null>(null);
	let visible = $state(false);
	let timeoutId: number | null = null;

	type Side = 'top' | 'bottom' | 'left' | 'right';
	type Placement = 'bottom-start' | Side;
	type Box = { width: number; height: number };
	type Rect = Box & { top: number; right: number; bottom: number; left: number };

	const OPPOSITE: Record<Side, Side> = {
		top: 'bottom',
		bottom: 'top',
		left: 'right',
		right: 'left'
	};

	const isVertical = (side: Side) => side === 'top' || side === 'bottom';

	/** Gap kept between the anchor and the box, on the placement's main axis. */
	const gapFor = (side: Side) => (isVertical(side) ? offset.y : offset.x);

	/** Room between the anchor and the screen edge on that side. */
	const spaceOn = (side: Side, anchorRect: Rect, viewport: Box) => {
		if (side === 'top') return anchorRect.top;
		if (side === 'bottom') return viewport.height - anchorRect.bottom;
		if (side === 'left') return anchorRect.left;
		return viewport.width - anchorRect.right;
	};

	const fitsOn = (side: Side, anchorRect: Rect, box: Box, viewport: Box) =>
		spaceOn(side, anchorRect, viewport) >=
		(isVertical(side) ? box.height : box.width) + gapFor(side) + SCREEN_MARGIN;

	/** Preferred side, else the opposite one, else whichever has more room. */
	const resolveSide = (preferred: Side, anchorRect: Rect, box: Box, viewport: Box): Side => {
		if (fitsOn(preferred, anchorRect, box, viewport)) return preferred;
		const opposite = OPPOSITE[preferred];
		if (fitsOn(opposite, anchorRect, box, viewport)) return opposite;
		return spaceOn(preferred, anchorRect, viewport) >= spaceOn(opposite, anchorRect, viewport)
			? preferred
			: opposite;
	};

	/** Centered on the chosen side of the anchor, one gap away from it. */
	const originOn = (side: Side, anchorRect: Rect, box: Box) => {
		const centerX = anchorRect.left + anchorRect.width / 2 - box.width / 2;
		const centerY = anchorRect.top + anchorRect.height / 2 - box.height / 2;
		if (side === 'top') return { x: centerX, y: anchorRect.top - box.height - offset.y };
		if (side === 'bottom') return { x: centerX, y: anchorRect.bottom + offset.y };
		if (side === 'left') return { x: anchorRect.left - box.width - offset.x, y: centerY };
		return { x: anchorRect.right + offset.x, y: centerY };
	};

	const clamp = (value: number, size: number, extent: number) =>
		Math.max(SCREEN_MARGIN, Math.min(value, extent - size - SCREEN_MARGIN));

	const calculatePosition = async () => {
		if (!boxElement) return;

		await tick(); // Wait for content to render
		if (!boxElement) return; // destroyed while awaiting

		const viewport = {
			width: window.innerWidth,
			height: window.innerHeight
		};

		// Never let the box itself be wider than the safe area: `width: fit-content`
		// otherwise measures a box that can't be nudged back inside the screen.
		maxWidth = Math.max(0, viewport.width - SCREEN_MARGIN * 2);

		// offsetWidth/offsetHeight, not getBoundingClientRect(): the box measures
		// itself while its `scale` intro transition is running, and the rect would
		// report the mid-animation size (~10% short) — which is exactly how much of
		// the right/bottom margin went missing.
		const box = { width: boxElement.offsetWidth, height: boxElement.offsetHeight };

		const anchorRect: Rect | undefined = anchor?.getBoundingClientRect();

		// Side placement: the anchor decides the main axis, so only the cross axis
		// is clamped to the screen. Clamping the main axis too is what used to slide
		// the box back over its own anchor when the preferred side was tight.
		if (anchorRect && placement !== 'bottom-start') {
			const side = resolveSide(placement, anchorRect, box, viewport);
			const origin = originOn(side, anchorRect, box);
			setPosition(
				isVertical(side) ? clamp(origin.x, box.width, viewport.width) : origin.x,
				isVertical(side) ? origin.y : clamp(origin.y, box.height, viewport.height)
			);
			return;
		}

		let x = position.x + offset.x;
		let y = position.y + offset.y;

		if (anchorRect) {
			x = anchorRect.left;
			y = anchorRect.bottom + offset.y;
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
				const aboveY = anchorRect.top - box.height - offset.y;
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

		setPosition(x, y);
	};

	// Only write when it actually moved: the resize observer re-measures often and
	// an identical object would still churn a re-render.
	const setPosition = (x: number, y: number) => {
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
		class:passthrough
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

	.floating-box.passthrough {
		pointer-events: none;
	}
</style>
