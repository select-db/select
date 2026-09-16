/**
 * Scrolls a container while something is dragged over its top or bottom edge,
 * so a row below the fold can be dropped on without letting go first.
 *
 * Driven here rather than left to the engine: Chromium scrolls a container
 * under a drag on its own, WebKit does not, and the desktop app is WebKit. The
 * e2e suite therefore cannot see the difference, which is why this was once
 * removed as redundant.
 *
 * Usage: <div class="scrollable" use:dragAutoScroll> ... </div>
 */

/** How close to an edge the pointer has to be, and the fastest it then scrolls. */
const EDGE_BAND_PX = 44;
const MAX_STEP_PX = 16;

/** Fastest at the very edge, down to a crawl at the inner border of the band. */
const stepFor = (distanceFromEdge: number) => {
	const depth = Math.max(0, Math.min(EDGE_BAND_PX, distanceFromEdge));
	return Math.max(1, Math.round(MAX_STEP_PX * (1 - depth / EDGE_BAND_PX)));
};

export function dragAutoScroll(node: HTMLElement) {
	let frame = 0;
	let step = 0;

	const tick = () => {
		node.scrollTop += step;
		frame = requestAnimationFrame(tick);
	};

	const stop = () => {
		if (frame) cancelAnimationFrame(frame);
		frame = 0;
		step = 0;
	};

	// In the capture phase, because a row's own dragover handler is free to stop
	// the event before it reaches the container.
	const onDragOver = (event: DragEvent) => {
		const box = node.getBoundingClientRect();
		const fromTop = event.clientY - box.top;
		const fromBottom = box.bottom - event.clientY;

		if (fromTop < EDGE_BAND_PX) step = -stepFor(fromTop);
		else if (fromBottom < EDGE_BAND_PX) step = stepFor(fromBottom);
		else return stop();

		if (!frame) frame = requestAnimationFrame(tick);
	};

	node.addEventListener('dragover', onDragOver, true);
	node.addEventListener('dragleave', stop, true);
	node.addEventListener('drop', stop, true);
	// On the document as well: a drag released outside the tree ends there, and
	// the loop would otherwise keep running with nothing being dragged.
	document.addEventListener('dragend', stop);
	document.addEventListener('drop', stop);

	return {
		destroy() {
			stop();
			node.removeEventListener('dragover', onDragOver, true);
			node.removeEventListener('dragleave', stop, true);
			node.removeEventListener('drop', stop, true);
			document.removeEventListener('dragend', stop);
			document.removeEventListener('drop', stop);
		}
	};
}
