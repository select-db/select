/**
 * scrollShadow shows a subtle inner shadow at the bottom of a scroll container
 * while there is more content to scroll to (i.e. scrollable and not at the
 * bottom). Pass `{ top: true }` to also show a shadow at the top while scrolled
 * away from the top. The shadows are `position: sticky` bands appended inside the
 * node, so they stay pinned to the viewport edges and work with virtual lists
 * (they don't depend on the parent layout or the content height).
 *
 * Usage: <div class="scrollable" use:scrollShadow> … </div>
 *        <div class="scrollable" use:scrollShadow={{ top: true }}> … </div>
 */
export function scrollShadow(node: HTMLElement, params: { top?: boolean } = {}) {
	// Visual styling (incl. theme-aware color) lives in the global
	// .scroll-shadow-band class in app.css; opacity is toggled here.
	const bottomOverlay = document.createElement('div');
	bottomOverlay.style.cssText =
		'position:sticky;bottom:0;left:0;right:0;height:0;z-index:4;pointer-events:none;';
	const bottomBand = document.createElement('div');
	bottomBand.className = 'scroll-shadow-band';
	bottomBand.style.opacity = '0';
	bottomOverlay.appendChild(bottomBand);

	let topBand: HTMLElement | null = null;
	let topOverlay: HTMLElement | null = null;
	if (params.top) {
		topOverlay = document.createElement('div');
		topOverlay.style.cssText =
			'position:sticky;top:0;left:0;right:0;height:0;z-index:4;pointer-events:none;';
		// Cancel the flex/grid gap the zero-height overlay would otherwise add
		// above the first item, so content stays flush with the top.
		const gap = parseFloat(getComputedStyle(node).rowGap) || 0;
		if (gap) topOverlay.style.marginBottom = `-${gap}px`;
		topBand = document.createElement('div');
		topBand.className = 'scroll-shadow-band scroll-shadow-band--top';
		topBand.style.opacity = '0';
		topOverlay.appendChild(topBand);
		node.insertBefore(topOverlay, node.firstChild);
	}

	node.appendChild(bottomOverlay);

	let frame = 0;
	let shownBottom = '0';
	let shownTop = '0';

	const update = () => {
		frame = 0;
		const scrollable = node.scrollHeight - node.clientHeight > 1;

		const atBottom = node.scrollTop + node.clientHeight >= node.scrollHeight - 1;
		const nextBottom = scrollable && !atBottom ? '1' : '0';
		// Only write when changed, otherwise the style mutation re-triggers the
		// MutationObserver and loops.
		if (nextBottom !== shownBottom) {
			shownBottom = nextBottom;
			bottomBand.style.opacity = nextBottom;
		}

		if (topBand) {
			const atTop = node.scrollTop <= 1;
			const nextTop = scrollable && !atTop ? '1' : '0';
			if (nextTop !== shownTop) {
				shownTop = nextTop;
				topBand.style.opacity = nextTop;
			}
		}
	};

	const schedule = () => {
		if (!frame) frame = requestAnimationFrame(update);
	};

	node.addEventListener('scroll', schedule, { passive: true });
	const ro = new ResizeObserver(schedule);
	ro.observe(node);
	// Catches content-height changes (e.g. virtual-list spacer, async data) that
	// don't change the container's own box size.
	const mo = new MutationObserver(schedule);
	mo.observe(node, { childList: true, subtree: true, attributes: true });
	schedule();

	return {
		destroy() {
			node.removeEventListener('scroll', schedule);
			ro.disconnect();
			mo.disconnect();
			if (frame) cancelAnimationFrame(frame);
			bottomOverlay.remove();
			topOverlay?.remove();
		}
	};
}
