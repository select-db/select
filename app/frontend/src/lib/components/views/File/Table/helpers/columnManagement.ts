export interface ColumnState {
	pinnedColumnIdx: Set<number>;
	columnWidths: number[];
}

export interface ColumnOffsets {
	pinned: Map<number, number>;
	nonPinned: number;
	lastPinnedIndex: number;
}

const resizeTimeouts = new Map<number, ReturnType<typeof setTimeout>>();

export function updateColumnWidth(
	columnIndex: number,
	width: number,
	state: ColumnState,
	onWidthChange: (widths: number[]) => void,
	immediate = false
) {
	const existing = resizeTimeouts.get(columnIndex);
	if (existing) clearTimeout(existing);

	const commitWidth = () => {
		const next = state.columnWidths.slice();
		if (next[columnIndex] === width) return;
		next[columnIndex] = width;
		onWidthChange(next);
	};

	if (immediate) {
		commitWidth();
	} else {
		resizeTimeouts.set(
			columnIndex,
			setTimeout(() => {
				commitWidth();
				resizeTimeouts.delete(columnIndex);
			}, 16)
		);
	}
}

export function createColumnMeasurer(
	getState: () => ColumnState,
	onWidthChange: (widths: number[]) => void
) {
	return (node: HTMLElement, columnIndex: number) => {
		const state = getState();
		updateColumnWidth(columnIndex, node.clientWidth, state, onWidthChange, true);

		let lastWidth = node.clientWidth;
		const resizeObserver = new ResizeObserver((entries) => {
			// Skip if width hasn't changed meaningfully (avoid scroll-triggered callbacks)
			const newWidth = Math.round(entries[0].contentRect.width);
			if (Math.abs(newWidth - lastWidth) < 1) return;
			lastWidth = newWidth;

			const state = getState();
			updateColumnWidth(columnIndex, newWidth, state, onWidthChange);
		});

		resizeObserver.observe(node);
		return {
			destroy() {
				resizeObserver.disconnect();
				const timeout = resizeTimeouts.get(columnIndex);
				if (timeout) {
					clearTimeout(timeout);
					resizeTimeouts.delete(columnIndex);
				}
			}
		};
	};
}

export function togglePinColumn(dataColumnIndex: number, state: ColumnState): Set<number> {
	const columnIndex = dataColumnIndex + 1;
	const newPinned = new Set(state.pinnedColumnIdx);
	if (newPinned.has(columnIndex)) {
		newPinned.delete(columnIndex);
	} else {
		newPinned.add(columnIndex);
	}
	newPinned.add(0);
	return newPinned;
}

export function isPinned(columnIndex: number, state: ColumnState) {
	return state.pinnedColumnIdx.has(columnIndex);
}

export function getSortedPinnedColumns(state: ColumnState) {
	return Array.from(state.pinnedColumnIdx).sort((a, b) => a - b);
}

export function getColumnOffsets(state: ColumnState): ColumnOffsets {
	const sortedPinnedColumns = getSortedPinnedColumns(state);
	const offsets = new Map<number, number>();
	let cumulativeOffset = 0;
	for (const colIndex of sortedPinnedColumns) {
		offsets.set(colIndex, cumulativeOffset);
		cumulativeOffset += state.columnWidths[colIndex] || 0;
	}
	return {
		pinned: offsets,
		nonPinned: cumulativeOffset,
		lastPinnedIndex: sortedPinnedColumns[sortedPinnedColumns.length - 1] ?? -1
	};
}

export function getColumnStyles(state: ColumnState, maxColumns: number): Map<number, string> {
	const styles = new Map<number, string>();
	const { pinned, nonPinned, lastPinnedIndex } = getColumnOffsets(state);
	for (let i = 0; i <= maxColumns; i++) {
		styles.set(
			i,
			pinned.has(i)
				? `left: ${pinned.get(i)}px;`
				: i > lastPinnedIndex
					? `left: ${nonPinned}px;`
					: 'left: 0;'
		);
	}
	return styles;
}

export function getColumnStyle(state: ColumnState, columnIndex: number, maxColumns: number) {
	const styles = getColumnStyles(state, maxColumns);
	return styles.get(columnIndex) || 'left: 0;';
}

const INDEX_COLUMN_WIDTH = 57;
const DEFAULT_COLUMN_WIDTH = 280;

export function resetColumnState(totalColumns?: number): ColumnState {
	const columnWidths =
		totalColumns != null && totalColumns > 0
			? [INDEX_COLUMN_WIDTH, ...Array(totalColumns - 1).fill(DEFAULT_COLUMN_WIDTH)]
			: [];
	return {
		pinnedColumnIdx: new Set([0]),
		columnWidths
	};
}

export function getColumnWidth(state: ColumnState, columnIndex: number): number {
	return (
		state.columnWidths[columnIndex] ??
		(columnIndex === 0 ? INDEX_COLUMN_WIDTH : DEFAULT_COLUMN_WIDTH)
	);
}

/** Left position (in px) of each column in the table. positions[0] = 0, positions[i] = sum of widths 0..i-1 */
export function getColumnPositions(state: ColumnState, totalColumns: number): number[] {
	const positions: number[] = [];
	let left = 0;
	for (let i = 0; i < totalColumns; i++) {
		positions.push(left);
		left += getColumnWidth(state, i);
	}
	positions.push(left); // end of last column
	return positions;
}

export const HORIZONTAL_OVERSCAN = 2;

/** Get sorted array of non-pinned column indices */
export function getNonPinnedColumns(state: ColumnState, totalColumns: number): number[] {
	const result: number[] = [];
	for (let i = 0; i < totalColumns; i++) {
		if (!state.pinnedColumnIdx.has(i)) {
			result.push(i);
		}
	}
	return result;
}

/** Visible range of non-pinned column indices (inclusive). All pinned columns are always rendered. */
export function getVisibleColumnRange(
	state: ColumnState,
	totalColumns: number,
	scrollLeft: number,
	viewportWidth: number
): { start: number; end: number } {
	const nonPinnedIndices = getNonPinnedColumns(state, totalColumns);

	if (nonPinnedIndices.length === 0) {
		// All columns are pinned
		return { start: totalColumns, end: totalColumns - 1 };
	}

	// Calculate where non-pinned columns start in the actual layout
	// Layout is: [pinned cols] [non-pinned cols]
	let pinnedWidth = 0;
	for (const idx of getSortedPinnedColumns(state)) {
		pinnedWidth += getColumnWidth(state, idx);
	}

	// Calculate positions for non-pinned columns in actual layout order
	const positions: number[] = [];
	let x = pinnedWidth;
	for (const idx of nonPinnedIndices) {
		positions.push(x);
		x += getColumnWidth(state, idx);
	}
	positions.push(x); // end position

	const scrollRight = scrollLeft + viewportWidth;

	// Viewport entirely before non-pinned area
	if (scrollRight <= pinnedWidth) {
		return { start: nonPinnedIndices[0], end: nonPinnedIndices[0] - 1 };
	}
	// Viewport entirely after table
	if (scrollLeft >= x) {
		return { start: nonPinnedIndices[0], end: nonPinnedIndices[0] - 1 };
	}

	// Find first visible non-pinned column
	let startArrayIdx = 0;
	for (let i = 0; i < nonPinnedIndices.length; i++) {
		const colRight = positions[i] + getColumnWidth(state, nonPinnedIndices[i]);
		if (colRight > scrollLeft) {
			startArrayIdx = Math.max(0, i - HORIZONTAL_OVERSCAN);
			break;
		}
	}

	// Find last visible non-pinned column
	let endArrayIdx = nonPinnedIndices.length - 1;
	for (let i = nonPinnedIndices.length - 1; i >= 0; i--) {
		if (positions[i] < scrollRight) {
			endArrayIdx = Math.min(nonPinnedIndices.length - 1, i + HORIZONTAL_OVERSCAN);
			break;
		}
	}

	// Return actual column indices
	return {
		start: nonPinnedIndices[startArrayIdx],
		end: nonPinnedIndices[endArrayIdx]
	};
}
