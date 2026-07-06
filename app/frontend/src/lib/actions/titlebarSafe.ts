import { isMac } from '$lib/utils/platform';

/**
 * Keeps interactive tab-bar elements clear of the OS window-control zone
 * (macOS traffic lights at top-left; custom window controls at top-right on
 * frameless platforms).
 */
const ZONE_LEFT = isMac ? 78 : 0;
const ZONE_RIGHT = 0;
const TOP_THRESHOLD = 40; // only the topmost row overlaps the native controls

export function titlebarSafe(node: HTMLElement, side: 'left' | 'right' = 'left') {
	const zone = side === 'left' ? ZONE_LEFT : ZONE_RIGHT;
	// Nothing to clear on this side/platform — skip the observer entirely.
	if (zone === 0) return {};

	const prop = side === 'left' ? 'padding-left' : 'padding-right';

	const update = () => {
		const r = node.getBoundingClientRect();
		// Lower rows in a vertical split don't collide with the native controls.
		if (r.top > TOP_THRESHOLD) {
			node.style.removeProperty(prop);
			return;
		}
		const gap = side === 'left' ? r.left : window.innerWidth - r.right;
		const pad = Math.max(0, zone - gap);
		if (pad > 0) node.style.setProperty(prop, `${pad}px`);
		else node.style.removeProperty(prop);
	};

	// `main`'s width changes on both sidebar drags and window resize — the one
	// signal that covers "when left and right bar width change". padding-left/right
	// live inside the border box, so they never shift the node's own rect: no loop.
	const target = node.closest('main') ?? document.body;
	const ro = new ResizeObserver(update);
	ro.observe(target);
	update();

	return { destroy: () => ro.disconnect() };
}
