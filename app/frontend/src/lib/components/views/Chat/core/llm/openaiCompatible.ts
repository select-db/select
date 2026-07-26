import type { ModelMessage, StreamChunk, StreamRequest } from './types';
import { parseSSE, readErrorMessage } from './sse';

/**
 * Chat Completions wire format, shared by every OpenAI-compatible provider
 * (OpenAI, xAI Grok, OpenRouter). Differences are limited to base URL, the
 * token-limit field name, and whether `temperature` is safe to send.
 */
export interface OpenAICompatConfig {
	baseURL: string;
	/** Reasoning models reject a custom temperature; omit it for them. */
	sendTemperature?: (model: string) => boolean;
	/** Field name for the output-token cap ('max_completion_tokens' | 'max_tokens'); omit to not send one. */
	maxTokensField?: string;
	extraHeaders?: Record<string, string>;
}

type ChatMessage =
	| { role: 'system' | 'user'; content: string }
	| {
			role: 'assistant';
			content: string | null;
			tool_calls?: Array<{ id: string; type: 'function'; function: { name: string; arguments: string } }>;
	  }
	| { role: 'tool'; tool_call_id: string; content: string };

function toChatMessages(messages: ModelMessage[], system?: string): ChatMessage[] {
	const out: ChatMessage[] = [];
	if (system) out.push({ role: 'system', content: system });

	for (const msg of messages) {
		if (msg.role === 'tool') {
			out.push({ role: 'tool', tool_call_id: msg.toolCallId ?? '', content: msg.content ?? '' });
		} else if (msg.role === 'user') {
			out.push({ role: 'user', content: msg.content ?? '' });
		} else {
			out.push({
				role: 'assistant',
				content: msg.content ?? '',
				...(msg.toolCalls?.length
					? {
							tool_calls: msg.toolCalls.map((tc) => ({
								id: tc.id,
								type: 'function' as const,
								function: { name: tc.function.name, arguments: tc.function.arguments }
							}))
						}
					: {})
			});
		}
	}
	return out;
}

export function createOpenAICompatStream(config: OpenAICompatConfig) {
	return async function* (req: StreamRequest): AsyncIterable<StreamChunk> {
		const sendTemp = config.sendTemperature ? config.sendTemperature(req.model) : true;

		const body: Record<string, unknown> = {
			model: req.model,
			messages: toChatMessages(req.messages, req.system),
			stream: true,
			stream_options: { include_usage: true },
			...(sendTemp && req.temperature != null ? { temperature: req.temperature } : {}),
			...(config.maxTokensField && req.maxTokens != null
				? { [config.maxTokensField]: req.maxTokens }
				: {}),
			...(req.tools?.length
				? {
						tools: req.tools.map((t) => ({
							type: 'function',
							function: { name: t.name, description: t.description, parameters: t.parameters }
						}))
					}
				: {}),
			...(req.modelOptions ?? {})
		};

		const response = await fetch(`${config.baseURL}/chat/completions`, {
			method: 'POST',
			headers: {
				'content-type': 'application/json',
				authorization: `Bearer ${req.apiKey}`,
				...(config.extraHeaders ?? {})
			},
			body: JSON.stringify(body),
			signal: req.signal
		});

		if (!response.ok) {
			yield { type: 'error', message: await readErrorMessage(response) };
			return;
		}

		// Chat Completions streams tool calls indexed within the delta; map index -> id.
		const toolIdByIndex = new Map<number, string>();

		for await (const event of parseSSE(response)) {
			const e = event as {
				choices?: Array<{
					delta?: {
						content?: string | null;
						reasoning?: string | null;
						reasoning_content?: string | null;
						tool_calls?: Array<{
							index?: number;
							id?: string;
							function?: { name?: string; arguments?: string };
						}>;
					};
					finish_reason?: string | null;
				}>;
				error?: { message?: string };
			};

			if (e.error) {
				yield { type: 'error', message: e.error.message ?? 'Provider stream error' };
				return;
			}

			const choice = e.choices?.[0];
			if (!choice) continue;
			const delta = choice.delta;

			if (delta?.content) yield { type: 'text', delta: delta.content };

			const reasoning = delta?.reasoning ?? delta?.reasoning_content;
			if (reasoning) yield { type: 'thinking', delta: reasoning };

			for (const tc of delta?.tool_calls ?? []) {
				const idx = tc.index ?? 0;
				if (tc.id) {
					toolIdByIndex.set(idx, tc.id);
					yield { type: 'tool-start', id: tc.id, name: tc.function?.name ?? '' };
				}
				if (tc.function?.arguments) {
					const id = toolIdByIndex.get(idx);
					if (id) yield { type: 'tool-args', id, delta: tc.function.arguments };
				}
			}

			if (choice.finish_reason) {
				yield {
					type: 'finish',
					finishReason: choice.finish_reason === 'tool_calls' ? 'tool_calls' : 'stop'
				};
			}
		}
	};
}
