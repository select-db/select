/**
 * scrollShadow shows a subtle inner shadow at the bottom of a scroll container
 * while there is more content to scroll to (i.e. scrollable and not at the
 * bottom). The shadow is a `position: sticky` band appended inside the node, so
 * it stays pinned to the bottom of the viewport and works with virtual lists
 * (it doesn't depend on the parent layout or the content height).
 *
 * Usage: <div class="scrollable" use:scrollShadow> … </div>
 */
export function scrollShadow(node: HTMLElement) {
	const overlay = document.createElement('div');
	overlay.style.cssText =
		'position:sticky;bottom:0;left:0;right:0;height:0;z-index:4;pointer-events:none;';

	// Visual styling (incl. theme-aware color) lives in the global
	// .scroll-shadow-band class in app.css; opacity is toggled here.
	const band = document.createElement('div');
	band.className = 'scroll-shadow-band';
	band.style.opacity = '0';
	overlay.appendChild(band);
	node.appendChild(overlay);

	let frame = 0;
	let shown = '0';

	const update = () => {
		frame = 0;
		const scrollable = node.scrollHeight - node.clientHeight > 1;
		const atBottom = node.scrollTop + node.clientHeight >= node.scrollHeight - 1;
		const next = scrollable && !atBottom ? '1' : '0';
		// Only write when changed, otherwise the style mutation re-triggers the
		// MutationObserver and loops.
		if (next !== shown) {
			shown = next;
			band.style.opacity = next;
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
			overlay.remove();
		}
	};
}
