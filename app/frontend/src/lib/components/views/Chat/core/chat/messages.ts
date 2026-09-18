import type { ModelMessage, ToolCall, ToolCallPart, UIMessage } from './types';

export function generateMessageId(): string {
	return `msg-${Date.now()}-${Math.random().toString(36).substring(7)}`;
}

/** A tool call is worth sending to the model once its arguments are finalized. */
function isToolCallReady(part: ToolCallPart): boolean {
	return (
		part.state === 'input-complete' ||
		part.state === 'approval-responded' ||
		part.output !== undefined
	);
}

function collapseText(parts: UIMessage['parts']): string | null {
	const text = parts
		.filter((p) => p.type === 'text')
		.map((p) => (p as { content: string }).content)
		.join('');
	return text.length > 0 ? text : null;
}

/**
 * Convert UI messages (parts-based) into the flat ModelMessage list providers
 * consume. System messages are dropped (the system prompt is passed separately).
 * Each assistant tool call is followed immediately by its tool result so the
 * strict user/assistant/tool ordering providers expect is preserved.
 */
export function convertMessagesToModelMessages(messages: UIMessage[]): ModelMessage[] {
	const out: ModelMessage[] = [];

	for (const msg of messages) {
		if (msg.role === 'system') continue;

		if (msg.role === 'user') {
			out.push({ role: 'user', content: collapseText(msg.parts) });
			continue;
		}

		// assistant
		const toolCalls: ToolCall[] = [];
		for (const part of msg.parts) {
			if (part.type === 'tool-call' && isToolCallReady(part)) {
				toolCalls.push({
					id: part.id,
					type: 'function',
					function: { name: part.name, arguments: part.arguments || '{}' }
				});
			}
		}

		out.push({
			role: 'assistant',
			content: collapseText(msg.parts),
			...(toolCalls.length ? { toolCalls } : {})
		});

		// Emit one tool result per call: prefer the tool-call part's output, fall
		// back to a matching tool-result part. Dedupe by tool call id.
		const emitted = new Set<string>();
		for (const part of msg.parts) {
			if (part.type === 'tool-call' && part.output !== undefined) {
				out.push({
					role: 'tool',
					content:
						typeof part.output === 'string' ? part.output : JSON.stringify(part.output),
					toolCallId: part.id
				});
				emitted.add(part.id);
			}
		}
		for (const part of msg.parts) {
			if (part.type === 'tool-result' && !emitted.has(part.toolCallId)) {
				out.push({ role: 'tool', content: part.content, toolCallId: part.toolCallId });
				emitted.add(part.toolCallId);
			}
		}
	}

	return out;
}
