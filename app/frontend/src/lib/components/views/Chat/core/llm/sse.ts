/**
 * Parse a `text/event-stream` (or newline-delimited JSON) response body into a
 * stream of parsed JSON payloads. Handles multi-line SSE frames and the
 * `data:` prefix; yields once per complete event. `[DONE]` sentinels are skipped.
 */
export async function* parseSSE(response: Response): AsyncGenerator<unknown> {
	const body = response.body;
	if (!body) return;

	const reader = body.getReader();
	const decoder = new TextDecoder();
	let buffer = '';

	try {
		for (;;) {
			const { done, value } = await reader.read();
			if (done) break;
			buffer += decoder.decode(value, { stream: true });

			// SSE events are separated by a blank line.
			let sep: number;
			while ((sep = indexOfBoundary(buffer)) !== -1) {
				const raw = buffer.slice(0, sep);
				buffer = buffer.slice(sep).replace(/^(\r?\n){1,2}/, '');
				const payload = extractData(raw);
				if (payload === null) continue;
				if (payload === '[DONE]') return;
				const parsed = tryParse(payload);
				if (parsed !== undefined) yield parsed;
			}
		}

		// Flush any trailing event without a terminating blank line.
		const payload = extractData(buffer);
		if (payload !== null && payload !== '[DONE]') {
			const parsed = tryParse(payload);
			if (parsed !== undefined) yield parsed;
		}
	} finally {
		reader.releaseLock();
	}
}

function indexOfBoundary(buffer: string): number {
	const lf = buffer.indexOf('\n\n');
	const crlf = buffer.indexOf('\r\n\r\n');
	if (lf === -1) return crlf;
	if (crlf === -1) return lf;
	return Math.min(lf, crlf);
}

/** Join the `data:` lines of one SSE frame; returns null when there is no data. */
function extractData(frame: string): string | null {
	const lines = frame.split(/\r?\n/);
	const dataLines: string[] = [];
	for (const line of lines) {
		const trimmed = line.trimStart();
		if (trimmed.startsWith('data:')) {
			dataLines.push(trimmed.slice(5).trimStart());
		} else if (trimmed && !trimmed.startsWith(':') && !trimmed.startsWith('event:')) {
			// Some endpoints emit bare JSON lines (no `data:` prefix).
			dataLines.push(trimmed);
		}
	}
	if (dataLines.length === 0) return null;
	return dataLines.join('\n');
}

function tryParse(payload: string): unknown {
	try {
		return JSON.parse(payload);
	} catch {
		return undefined;
	}
}

/** Read an error response body and surface a concise provider error message. */
export async function readErrorMessage(response: Response): Promise<string> {
	const text = await response.text().catch(() => '');
	if (text) {
		try {
			const data = JSON.parse(text) as { error?: { message?: string } | string; message?: string };
			const err = data?.error;
			const msg = typeof err === 'string' ? err : (err?.message ?? data?.message);
			if (typeof msg === 'string' && msg.length > 0) return msg;
		} catch {
			// fall through to raw text
		}
		return text.length > 400 ? text.slice(0, 400) + '…' : text;
	}
	return `${response.status} ${response.statusText}`.trim();
}
