/**
 * scrollShadow shows a subtle inner shadow at the trailing edge of a scroll
 * container while there is more content to scroll to (i.e. scrollable and not at
 * the end). Pass `{ top: true }` to also show a shadow at the leading edge while
 * scrolled away from the start.
 *
 * The axis defaults to vertical (`'y'`: bottom/top shadows). Pass `{ axis: 'x' }`
 * for a horizontally scrolling container (right/left shadows); there `top: true`
 * enables the leading (left) shadow.
 *
 * The shadows are `position: sticky` bands appended inside the node, so they stay
 * pinned to the viewport edges and work with virtual lists (they don't depend on
 * the parent layout or the content size).
 *
 * Usage: <div class="scrollable" use:scrollShadow> … </div>
 *        <div class="scrollable" use:scrollShadow={{ top: true }}> … </div>
 *        <div class="row" use:scrollShadow={{ axis: 'x', top: true }}> … </div>
 */
export function scrollShadow(
	node: HTMLElement,
	params: { top?: boolean; axis?: 'x' | 'y' } = {}
) {
	const horizontal = params.axis === 'x';

	// Visual styling (incl. theme-aware color) lives in the global
	// .scroll-shadow-band class in app.css; opacity is toggled here. The zero-size
	// sticky overlay pins the band to a viewport edge without taking layout space.
	const gap =
		parseFloat(horizontal ? getComputedStyle(node).columnGap : getComputedStyle(node).rowGap) || 0;

	// Trailing edge (bottom for vertical, right for horizontal) — always present.
	const trailingOverlay = document.createElement('div');
	trailingOverlay.style.cssText = horizontal
		? 'position:sticky;top:0;bottom:0;right:0;width:0;z-index:4;pointer-events:none;'
		: 'position:sticky;bottom:0;left:0;right:0;height:0;z-index:4;pointer-events:none;';
	// Cancel the flex/grid gap the zero-size overlay would otherwise add after the
	// last item (horizontal only; the vertical path keeps its original behaviour).
	if (horizontal && gap) trailingOverlay.style.marginLeft = `-${gap}px`;
	const trailingBand = document.createElement('div');
	trailingBand.className = horizontal
		? 'scroll-shadow-band scroll-shadow-band--right'
		: 'scroll-shadow-band';
	trailingBand.style.opacity = '0';
	trailingOverlay.appendChild(trailingBand);

	// Leading edge (top for vertical, left for horizontal) — opt-in.
	let leadingBand: HTMLElement | null = null;
	let leadingOverlay: HTMLElement | null = null;
	if (params.top) {
		leadingOverlay = document.createElement('div');
		leadingOverlay.style.cssText = horizontal
			? 'position:sticky;top:0;bottom:0;left:0;width:0;z-index:4;pointer-events:none;'
			: 'position:sticky;top:0;left:0;right:0;height:0;z-index:4;pointer-events:none;';
		// Cancel the gap the zero-size overlay would add before the first item, so
		// content stays flush with the leading edge.
		if (gap) leadingOverlay.style[horizontal ? 'marginRight' : 'marginBottom'] = `-${gap}px`;
		leadingBand = document.createElement('div');
		leadingBand.className = horizontal
			? 'scroll-shadow-band scroll-shadow-band--left'
			: 'scroll-shadow-band scroll-shadow-band--top';
		leadingBand.style.opacity = '0';
		leadingOverlay.appendChild(leadingBand);
		node.insertBefore(leadingOverlay, node.firstChild);
	}

	node.appendChild(trailingOverlay);

	let frame = 0;
	let shownTrailing = '0';
	let shownLeading = '0';

	const update = () => {
		frame = 0;
		const scrollable = horizontal
			? node.scrollWidth - node.clientWidth > 1
			: node.scrollHeight - node.clientHeight > 1;

		const atEnd = horizontal
			? node.scrollLeft + node.clientWidth >= node.scrollWidth - 1
			: node.scrollTop + node.clientHeight >= node.scrollHeight - 1;
		const nextTrailing = scrollable && !atEnd ? '1' : '0';
		// Only write when changed, otherwise the style mutation re-triggers the
		// MutationObserver and loops.
		if (nextTrailing !== shownTrailing) {
			shownTrailing = nextTrailing;
			trailingBand.style.opacity = nextTrailing;
		}

		if (leadingBand) {
			const atStart = horizontal ? node.scrollLeft <= 1 : node.scrollTop <= 1;
			const nextLeading = scrollable && !atStart ? '1' : '0';
			if (nextLeading !== shownLeading) {
				shownLeading = nextLeading;
				leadingBand.style.opacity = nextLeading;
			}
		}
	};

	const schedule = () => {
		if (!frame) frame = requestAnimationFrame(update);
	};

	node.addEventListener('scroll', schedule, { passive: true });
	const ro = new ResizeObserver(schedule);
	ro.observe(node);
	// Catches content-size changes (e.g. virtual-list spacer, async data) that
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
			trailingOverlay.remove();
			leadingOverlay?.remove();
		}
	};
}
