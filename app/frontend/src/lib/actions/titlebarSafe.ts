import { get } from 'svelte/store';
import { osStore } from '$lib/utils/platform';

/**
 * Keeps interactive tab-bar elements clear of the OS window-control zone
 * (macOS traffic lights at top-left; custom window controls at top-right on
 * frameless platforms).
 */
const ZONE_LEFT = 78;
const ZONE_RIGHT = 0;
const TOP_THRESHOLD = 40; // only the topmost row overlaps the native controls

export function titlebarSafe(node: HTMLElement, side: 'left' | 'right' = 'left') {
	// The traffic lights are macOS's, and which platform this is comes from the
	// backend a moment after the first render -- so the zone is read when it is
	// needed rather than fixed at import.
	const zoneFor = (os: string) => (side === 'left' && os === 'macos' ? ZONE_LEFT : ZONE_RIGHT);

	const prop = side === 'left' ? 'padding-left' : 'padding-right';

	const update = () => {
		const zone = zoneFor(get(osStore));
		if (zone === 0) {
			node.style.removeProperty(prop);
			return;
		}

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
	const unsubscribe = osStore.subscribe(update);

	return {
		destroy: () => {
			ro.disconnect();
			unsubscribe();
		}
	};
}
