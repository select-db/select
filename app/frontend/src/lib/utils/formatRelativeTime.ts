// "3h ago" style formatting for API-key timestamps. Returns 'Never' for null.
export function formatRelativeTime(value: string | null | undefined): string {
	if (!value) return 'Never';
	const then = new Date(value);
	if (isNaN(then.getTime())) return 'Never';

	const seconds = Math.floor((Date.now() - then.getTime()) / 1000);
	if (seconds < 60) return 'just now';
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.floor(hours / 24);
	if (days < 30) return `${days}d ago`;
	return then.toLocaleDateString();
}

// Absolute date for expiry. null = no expiry.
export function formatExpiry(value: string | null | undefined): string {
	if (!value) return 'Never';
	const d = new Date(value);
	if (isNaN(d.getTime())) return 'Never';
	const label = d.toLocaleDateString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
	return d.getTime() < Date.now() ? `Expired (${label})` : label;
}
