/**
 * Read a pixel width persisted in localStorage.
 *
 * Anything unparseable — a missing key, a stale non-numeric value — falls back
 * to `fallback` rather than leaking `NaN` into an inline `width: ${…}px` style,
 * where the browser would drop the declaration and let the bar size itself to
 * its content.
 */
export function readStoredWidth(key: string, fallback: number): number {
	const saved = parseInt(localStorage.getItem(key) ?? '', 10);
	return Number.isFinite(saved) ? saved : fallback;
}
