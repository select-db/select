export function formatMs(ms: number) {
	const pad = (n: number, z = 2) => ('00' + n).slice(-z);

	const m = Math.floor(ms / 60000);
	const s = Math.floor((ms % 60000) / 1000);
	const msPart = Math.floor(ms % 1000);

	return `${pad(m)}:${pad(s)}.${pad(msPart, 3)}`;
}
