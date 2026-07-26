import type { ModelMessage, ProviderStream, StreamChunk, StreamRequest } from './types';
import { parseSSE, readErrorMessage } from './sse';

const API_URL = 'https://api.anthropic.com/v1/messages';
const API_VERSION = '2023-06-01';
const DEFAULT_MAX_TOKENS = 4096;

type AnthropicBlock =
	| { type: 'text'; text: string }
	| { type: 'tool_use'; id: string; name: string; input: unknown }
	| { type: 'tool_result'; tool_use_id: string; content: string };

type AnthropicMessage = { role: 'user' | 'assistant'; content: AnthropicBlock[] };

function toAnthropicMessages(messages: ModelMessage[]): AnthropicMessage[] {
	const out: AnthropicMessage[] = [];

	const push = (role: 'user' | 'assistant', blocks: AnthropicBlock[]) => {
		if (blocks.length === 0) return;
		const last = out[out.length - 1];
		// Anthropic requires strict user/assistant alternation; merge same-role runs.
		if (last && last.role === role) last.content.push(...blocks);
		else out.push({ role, content: blocks });
	};

	for (const msg of messages) {
		if (msg.role === 'tool') {
			push('user', [
				{ type: 'tool_result', tool_use_id: msg.toolCallId ?? '', content: msg.content ?? '' }
			]);
			continue;
		}
		if (msg.role === 'user') {
			if (msg.content) push('user', [{ type: 'text', text: msg.content }]);
			continue;
		}
		// assistant
		const blocks: AnthropicBlock[] = [];
		if (msg.content) blocks.push({ type: 'text', text: msg.content });
		for (const tc of msg.toolCalls ?? []) {
			blocks.push({
				type: 'tool_use',
				id: tc.id,
				name: tc.function.name,
				input: safeParse(tc.function.arguments)
			});
		}
		push('assistant', blocks);
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

export const streamAnthropic: ProviderStream = async function* (
	req: StreamRequest
): AsyncIterable<StreamChunk> {
	const body: Record<string, unknown> = {
		model: req.model,
		max_tokens: req.maxTokens ?? DEFAULT_MAX_TOKENS,
		messages: toAnthropicMessages(req.messages),
		stream: true,
		...(req.system ? { system: req.system } : {}),
		...(req.temperature != null ? { temperature: req.temperature } : {}),
		...(req.tools?.length
			? {
					tools: req.tools.map((t) => ({
						name: t.name,
						description: t.description,
						input_schema: t.parameters
					}))
				}
			: {}),
		...(req.modelOptions ?? {})
	};

	const response = await fetch(API_URL, {
		method: 'POST',
		headers: {
			'content-type': 'application/json',
			'x-api-key': req.apiKey,
			'anthropic-version': API_VERSION,
			'anthropic-dangerous-direct-browser-access': 'true'
		},
		body: JSON.stringify(body),
		signal: req.signal
	});

	if (!response.ok) {
		yield { type: 'error', message: await readErrorMessage(response) };
		return;
	}

	// Map content-block index -> tool call id (for input_json_delta routing).
	const toolIdByIndex = new Map<number, string>();

	for await (const event of parseSSE(response)) {
		const e = event as {
			type?: string;
			index?: number;
			content_block?: { type?: string; id?: string; name?: string };
			delta?: {
				type?: string;
				text?: string;
				partial_json?: string;
				thinking?: string;
				stop_reason?: string;
			};
			error?: { message?: string };
		};

		switch (e.type) {
			case 'content_block_start': {
				const block = e.content_block;
				if (block?.type === 'tool_use' && block.id) {
					toolIdByIndex.set(e.index ?? 0, block.id);
					yield { type: 'tool-start', id: block.id, name: block.name ?? '' };
				}
				break;
			}
			case 'content_block_delta': {
				const d = e.delta;
				if (d?.type === 'text_delta' && d.text) {
					yield { type: 'text', delta: d.text };
				} else if (d?.type === 'thinking_delta' && d.thinking) {
					yield { type: 'thinking', delta: d.thinking };
				} else if (d?.type === 'input_json_delta' && d.partial_json != null) {
					const id = toolIdByIndex.get(e.index ?? 0);
					if (id) yield { type: 'tool-args', id, delta: d.partial_json };
				}
				break;
			}
			case 'message_delta': {
				const reason = e.delta?.stop_reason;
				if (reason) {
					yield { type: 'finish', finishReason: reason === 'tool_use' ? 'tool_calls' : 'stop' };
				}
				break;
			}
			case 'error':
				yield { type: 'error', message: e.error?.message ?? 'Anthropic stream error' };
				return;
		}
	}
};
