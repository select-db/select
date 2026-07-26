import type { ModelMessage, ProviderStream, StreamChunk, StreamRequest } from './types';
import { parseSSE, readErrorMessage } from './sse';

const BASE_URL = 'https://generativelanguage.googleapis.com/v1beta/models';

type GeminiPart =
	| { text: string; thought?: boolean }
	| { functionCall: { name: string; args: unknown } }
	| { functionResponse: { name: string; response: { content: string } } };

type GeminiContent = { role: 'user' | 'model'; parts: GeminiPart[] };

/**
 * Gemini matches tool results to calls by function NAME (there are no call ids
 * on the wire), so we resolve each tool message's id back to the name the
 * assistant used.
 */
function toGeminiContents(messages: ModelMessage[]): GeminiContent[] {
	const nameByToolCallId = new Map<string, string>();
	for (const msg of messages) {
		for (const tc of msg.toolCalls ?? []) nameByToolCallId.set(tc.id, tc.function.name);
	}

	const out: GeminiContent[] = [];
	const push = (role: 'user' | 'model', parts: GeminiPart[]) => {
		if (parts.length === 0) return;
		const last = out[out.length - 1];
		if (last && last.role === role) last.parts.push(...parts);
		else out.push({ role, parts });
	};

	for (const msg of messages) {
		if (msg.role === 'tool') {
			const name = nameByToolCallId.get(msg.toolCallId ?? '') ?? msg.toolCallId ?? '';
			push('user', [{ functionResponse: { name, response: { content: msg.content ?? '' } } }]);
		} else if (msg.role === 'user') {
			if (msg.content) push('user', [{ text: msg.content }]);
		} else {
			const parts: GeminiPart[] = [];
			if (msg.content) parts.push({ text: msg.content });
			for (const tc of msg.toolCalls ?? []) {
				parts.push({ functionCall: { name: tc.function.name, args: safeParse(tc.function.arguments) } });
			}
			push('model', parts);
		}
	}
	return out;
}

function safeParse(json: string): unknown {
	try {
		return json ? JSON.parse(json) : {};
	} catch {
		return {};
	}
}

/** Gemini's schema validator rejects `$schema`/`additionalProperties`; strip them recursively. */
function sanitizeGeminiSchema(schema: unknown): unknown {
	if (Array.isArray(schema)) return schema.map(sanitizeGeminiSchema);
	if (schema && typeof schema === 'object') {
		const out: Record<string, unknown> = {};
		for (const [key, value] of Object.entries(schema)) {
			if (key === '$schema' || key === 'additionalProperties') continue;
			out[key] = sanitizeGeminiSchema(value);
		}
		return out;
	}
	return schema;
}

export const streamGemini: ProviderStream = async function* (
	req: StreamRequest
): AsyncIterable<StreamChunk> {
	const generationConfig: Record<string, unknown> = {
		...(req.temperature != null ? { temperature: req.temperature } : {}),
		...(req.maxTokens != null ? { maxOutputTokens: req.maxTokens } : {}),
		...(req.modelOptions ?? {})
	};

	const body: Record<string, unknown> = {
		contents: toGeminiContents(req.messages),
		...(req.system ? { systemInstruction: { parts: [{ text: req.system }] } } : {}),
		...(req.tools?.length
			? {
					tools: [
						{
							functionDeclarations: req.tools.map((t) => ({
								name: t.name,
								description: t.description,
								parameters: sanitizeGeminiSchema(t.parameters)
							}))
						}
					]
				}
			: {}),
		...(Object.keys(generationConfig).length ? { generationConfig } : {})
	};

	const url = `${BASE_URL}/${encodeURIComponent(req.model)}:streamGenerateContent?alt=sse`;
	const response = await fetch(url, {
		method: 'POST',
		headers: { 'content-type': 'application/json', 'x-goog-api-key': req.apiKey },
		body: JSON.stringify(body),
		signal: req.signal
	});

	if (!response.ok) {
		yield { type: 'error', message: await readErrorMessage(response) };
		return;
	}

	let sawToolCall = false;

	for await (const event of parseSSE(response)) {
		const e = event as {
			candidates?: Array<{
				content?: { parts?: Array<{ text?: string; thought?: boolean; functionCall?: { id?: string; name?: string; args?: unknown } }> };
				finishReason?: string;
			}>;
			error?: { message?: string };
		};

		if (e.error) {
			yield { type: 'error', message: e.error.message ?? 'Gemini stream error' };
			return;
		}

		const candidate = e.candidates?.[0];
		for (const part of candidate?.content?.parts ?? []) {
			if (part.functionCall) {
				const id = part.functionCall.id ?? `call_${part.functionCall.name}_${sawToolCall ? '1' : '0'}`;
				sawToolCall = true;
				yield { type: 'tool-start', id, name: part.functionCall.name ?? '' };
				yield { type: 'tool-args', id, delta: JSON.stringify(part.functionCall.args ?? {}) };
			} else if (typeof part.text === 'string' && part.text.length > 0) {
				if (part.thought) yield { type: 'thinking', delta: part.text };
				else yield { type: 'text', delta: part.text };
			}
		}

		if (candidate?.finishReason) {
			yield { type: 'finish', finishReason: sawToolCall ? 'tool_calls' : 'stop' };
		}
	}
};
