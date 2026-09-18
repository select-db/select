/** The text a result cell shows for a raw value. */
export function formatCellValue(value: unknown): string {
	if (value === null) return 'NULL';
	if (value === undefined) return '';
	return typeof value === 'string' ? value : String(value);
}

export interface TextMeasurer {
	/** Rendered width of `text` in the table's cell font, in px. */
	measure(text: string): number;
	/** Horizontal padding a cell puts around its text, in px. */
	cellPadding: number;
}

// Rough enough to keep column sizing from crashing where there is no DOM or no
// 2d context; the widths it produces are only ever a fallback.
const FALLBACK_CHAR_WIDTH = 7;
const FALLBACK_CELL_PADDING = 16;
const fallbackMeasure = (text: string) => text.length * FALLBACK_CHAR_WIDTH;

/**
 * Measure text the way the results table renders it.
 *
 * The font size, weight and letter spacing of a cell come from theme variables
 * the user can change, so they cannot be restated here. A bare <span> under
 * <body> inherits exactly what a cell's <span> inherits, so we read those back
 * off a throwaway one, in px, and hand them to a canvas.
 */
export function createTextMeasurer(): TextMeasurer {
	if (typeof document === 'undefined') {
		return { measure: fallbackMeasure, cellPadding: FALLBACK_CELL_PADDING };
	}

	const probe = document.createElement('span');
	probe.style.position = 'absolute';
	probe.style.visibility = 'hidden';
	// The declaration `td > .text-cell` and `th span` carry in ResultsTable.
	probe.style.padding = 'var(--space-sm-md) var(--space-sm)';
	document.body.appendChild(probe);

	const style = getComputedStyle(probe);
	// The `font` shorthand reads back empty in some engines; build it.
	const font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
	const letterSpacing = parseFloat(style.letterSpacing) || 0;
	const cellPadding = (parseFloat(style.paddingLeft) || 0) + (parseFloat(style.paddingRight) || 0);
	probe.remove();

	const ctx = document.createElement('canvas').getContext('2d');
	if (!ctx) return { measure: fallbackMeasure, cellPadding };
	ctx.font = font;

	return {
		// Not every engine folds letter-spacing into measureText, and the ones
		// that do need it set on the context, so add it per character instead.
		measure: (text) => ctx.measureText(text).width + letterSpacing * text.length,
		cellPadding
	};
}
