export const hslToHex = (color: string): string | undefined => {
	if (!color) return undefined;

	const trimmed = color.trim();

	// If already a hex color, return as-is
	if (trimmed.startsWith('#')) return trimmed;

	// If it's an rgb/rgba value, convert to hex
	const rgbMatch = trimmed.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
	if (rgbMatch) {
		const [, r, g, b] = rgbMatch.map(Number);
		return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`;
	}

	// Parse HSL value
	const hslMatch = trimmed.match(/hsl\(\s*([\d.]+)\s*,?\s*([\d.]+)%?\s*,?\s*([\d.]+)%?\s*\)/i);
	if (!hslMatch) {
		// Try simple number extraction as fallback (e.g., "210, 12%, 98%")
		const numMatch = trimmed.match(/[\d.]+/g);
		if (!numMatch || numMatch.length < 3) return undefined;
		const [h, s, l] = numMatch.map(Number);
		return hslToHexFromValues(h, s, l);
	}

	const [, h, s, l] = hslMatch.map(Number);
	return hslToHexFromValues(h, s, l);
};

function hslToHexFromValues(h: number, s: number, l: number): string {
	h = h / 360;
	s = s / 100;
	l = l / 100;

	const a = s * Math.min(l, 1 - l);
	const f = (n: number) => {
		const k = (n + h * 12) % 12;
		const color = l - a * Math.max(Math.min(k - 3, 9 - k, 1), -1);
		return Math.round(255 * color);
	};
	return `#${f(0).toString(16).padStart(2, '0')}${f(8).toString(16).padStart(2, '0')}${f(4).toString(16).padStart(2, '0')}`;
}
