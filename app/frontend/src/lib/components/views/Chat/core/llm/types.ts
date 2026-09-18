/**
 * Minimal LLM provider layer.
 *
 * Each provider is a plain async generator that makes ONE browser `fetch` to the
 * vendor's REST API and streams a single assistant turn back as a normalized
 * {@link StreamChunk}. No vendor SDKs, no Node built-ins — everything runs in the
 * browser. The chat state machine ({@link ../chat/chat-client}) layers the
 * multi-turn tool loop on top.
 */

/** Normalized message shape passed to providers (provider-agnostic). */
export interface ToolCall {
	id: string;
	type: 'function';
	function: { name: string; arguments: string };
}

export interface ModelMessage {
	role: 'user' | 'assistant' | 'tool';
	content: string | null;
	toolCalls?: ToolCall[];
	toolCallId?: string;
}

/** A tool advertised to the model. `parameters` is a JSON Schema object. */
export interface LlmTool {
	name: string;
	description: string;
	parameters: Record<string, unknown>;
}

export type FinishReason = 'stop' | 'length' | 'tool_calls' | 'content_filter' | null;

/** Normalized streaming events emitted by every provider. */
export type StreamChunk =
	| { type: 'text'; delta: string }
	| { type: 'thinking'; delta: string }
	| { type: 'tool-start'; id: string; name: string }
	| { type: 'tool-args'; id: string; delta: string }
	| { type: 'finish'; finishReason: FinishReason }
	| { type: 'error'; message: string };

export interface StreamRequest {
	model: string;
	apiKey: string;
	system?: string;
	messages: ModelMessage[];
	tools?: LlmTool[];
	temperature?: number;
	maxTokens?: number;
	/** Provider-specific extra body fields (e.g. reasoning/thinking config). */
	modelOptions?: Record<string, unknown>;
	signal?: AbortSignal;
}

export type ProviderStream = (req: StreamRequest) => AsyncIterable<StreamChunk>;
