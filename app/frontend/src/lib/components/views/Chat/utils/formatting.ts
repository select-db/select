export function toolLabel(name: string): string {
	return name
		.split('_')
		.map((w) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase())
		.join(' ');
}

export function truncate(text: string, max = 8000): string {
	if (text.length <= max) return text;
	return text.slice(0, max) + '\n\n...[truncated]';
}

export function prettyJson(value: string | unknown): string {
	if (value == null) return '';
	const str = typeof value === 'string' ? value : JSON.stringify(value);
	const trimmed = str.trim();
	if (!trimmed) return '';
	try {
		const parsed = JSON.parse(trimmed);
		return JSON.stringify(parsed, null, 2);
	} catch {
		return str;
	}
}

export function highlightJson(value: string | unknown): string {
	return prettyJson(value);
}
