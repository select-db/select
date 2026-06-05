import { tick } from 'svelte';

export const MIN_HEIGHT = 40;
export const DEFAULT_ROW_HEIGHT = 41;
export const VIRTUAL_OVERSCAN = 15;

export interface VisibleRange {
	start: number;
	end: number;
}

export interface VirtualScrollState {
	visibleRange: VisibleRange;
	viewportHeight: number;
	viewportWidth: number;
	scrollLeft: number;
	rowHeight: number;
	scrollContainer: HTMLDivElement | null;
}

export function updateVisibleRange(
	state: VirtualScrollState,
	totalRows: number,
	overrideScrollTop?: number
): VisibleRange | null {
	if (!state.scrollContainer || !totalRows) return null;

	const container = state.scrollContainer;
	const effectiveScrollTop = overrideScrollTop ?? container.scrollTop;
	const height = state.viewportHeight || container.clientHeight || 1;
	const effectiveRowHeight = state.rowHeight || DEFAULT_ROW_HEIGHT;
	const start = Math.max(Math.floor(effectiveScrollTop / effectiveRowHeight) - VIRTUAL_OVERSCAN, 0);
	const visibleCount = Math.ceil(height / effectiveRowHeight) + VIRTUAL_OVERSCAN * 2;
	const end = Math.min(start + visibleCount, totalRows);

	if (state.visibleRange.start !== start || state.visibleRange.end !== end) {
		return { start, end };
	}
	return null;
}

export function createScrollHandler(
	getState: () => VirtualScrollState,
	getTotalRows: () => number,
	onRangeChange: (range: VisibleRange) => void,
	onStateChange: (updates: Partial<VirtualScrollState>) => void
) {
	let rafId: number | null = null;
	let pendingScrollTop: number | null = null;
	let pendingNode: HTMLElement | null = null;

	return (event: Event) => {
		const target = event.currentTarget as HTMLElement;
		pendingScrollTop = target.scrollTop;
		pendingNode = target;

		// Batch scroll updates with requestAnimationFrame
		if (rafId !== null) return;

		rafId = requestAnimationFrame(() => {
			rafId = null;
			if (pendingScrollTop === null || pendingNode === null) return;

			onStateChange({ scrollLeft: pendingNode.scrollLeft });

			const state = getState();
			const newRange = updateVisibleRange(state, getTotalRows(), pendingScrollTop);
			pendingScrollTop = null;
			pendingNode = null;

			if (newRange) {
				onRangeChange(newRange);
			}
		});
	};
}

export function createViewportObserver(
	getState: () => VirtualScrollState,
	getTotalRows: () => number,
	onStateChange: (updates: Partial<VirtualScrollState>) => void,
	onRangeChange: (range: VisibleRange) => void
) {
	const handleScroll = createScrollHandler(
		getState,
		getTotalRows,
		onRangeChange,
		onStateChange
	);

	return (node: HTMLElement) => {
		onStateChange({
			scrollContainer: node as HTMLDivElement,
			viewportHeight: node.clientHeight,
			viewportWidth: node.clientWidth,
			scrollLeft: node.scrollLeft
		});

		const state = getState();
		const newRange = updateVisibleRange(state, getTotalRows(), node.scrollTop);
		if (newRange) {
			onRangeChange(newRange);
		}

		const resizeObserver = new ResizeObserver(() => {
			onStateChange({
				viewportHeight: node.clientHeight,
				viewportWidth: node.clientWidth
			});
			const state = getState();
			const newRange = updateVisibleRange(state, getTotalRows());
			if (newRange) {
				onRangeChange(newRange);
			}
		});
		resizeObserver.observe(node);

		node.addEventListener('scroll', handleScroll, { passive: true });

		return {
			destroy() {
				node.removeEventListener('scroll', handleScroll);
				resizeObserver.disconnect();
				const state = getState();
				if (state.scrollContainer === node) {
					onStateChange({ scrollContainer: null });
				}
			}
		};
	};
}

export function createRowMeasurer(
	getState: () => VirtualScrollState,
	getTotalRows: () => number,
	onStateChange: (updates: Partial<VirtualScrollState>) => void,
	onRangeChange: (range: VisibleRange) => void
) {
	return (node: HTMLTableRowElement, shouldMeasure: boolean) => {
		if (!node) return;
		let observer: ResizeObserver | null = null;

		const attach = () => {
			if (!shouldMeasure || observer) return;
			observer = new ResizeObserver((entries) => {
				const entry = entries[0];
				const nextHeight = Math.max(Math.round(entry.contentRect.height), 1);
				const state = getState();
				if (nextHeight && Math.abs(nextHeight - state.rowHeight) > 0.5) {
					onStateChange({ rowHeight: nextHeight });
					const newRange = updateVisibleRange(state, getTotalRows());
					if (newRange) {
						onRangeChange(newRange);
					}
				}
			});
			observer.observe(node);
		};

		const detach = () => {
			if (!observer) return;
			observer.disconnect();
			observer = null;
		};

		attach();

		return {
			update(nextShouldMeasure: boolean) {
				shouldMeasure = nextShouldMeasure;
				if (!shouldMeasure) {
					detach();
					return;
				}
				attach();
			},
			destroy() {
				detach();
			}
		};
	};
}

export function getVisibleRows(allRows: unknown[][], visibleRange: VisibleRange) {
	if (!allRows?.length) return [];
	const { start, end } = visibleRange;
	if (end <= start) return [];
	return allRows.slice(start, end);
}

export function getTopSpacerHeight(visibleRange: VisibleRange, rowHeight: number) {
	return visibleRange.start * rowHeight;
}

export function getBottomSpacerHeight(
	totalRows: number,
	visibleRange: VisibleRange,
	rowHeight: number
) {
	if (!totalRows) return 0;
	return Math.max(totalRows - visibleRange.end, 0) * rowHeight;
}

export async function resetVisibleRange(
	state: VirtualScrollState,
	totalRows: number
): Promise<VisibleRange> {
	if (!totalRows) {
		return { start: 0, end: 0 };
	}

	await tick();
	if (state.scrollContainer) {
		const effectiveScrollTop = state.scrollContainer.scrollTop;
		const height = state.viewportHeight || state.scrollContainer.clientHeight || 1;
		const effectiveRowHeight = state.rowHeight || DEFAULT_ROW_HEIGHT;
		const start = Math.max(
			Math.floor(effectiveScrollTop / effectiveRowHeight) - VIRTUAL_OVERSCAN,
			0
		);
		const visibleCount = Math.ceil(height / effectiveRowHeight) + VIRTUAL_OVERSCAN * 2;
		const end = Math.min(start + visibleCount, totalRows);
		return { start, end };
	}
	const fallbackVisible = Math.min(totalRows, 50);
	return { start: 0, end: fallbackVisible };
}
