import type { StreamChunk, ModelMessage, ToolCall } from '../llm';

export type { StreamChunk, ModelMessage, ToolCall };

/** Lifecycle state of a single tool call, from streamed args to a final decision. */
export type ToolCallState =
	| 'awaiting-input'
	| 'input-streaming'
	| 'input-complete'
	| 'approval-requested'
	| 'approval-responded';

export type ToolResultState = 'streaming' | 'complete' | 'error';

export type ChatClientState = 'ready' | 'submitted' | 'streaming' | 'error';

export interface TextPart {
	type: 'text';
	content: string;
}

export interface ThinkingPart {
	type: 'thinking';
	content: string;
}

export interface ToolCallPart {
	type: 'tool-call';
	id: string;
	name: string;
	/** JSON string of the tool arguments (may be partial while streaming). */
	arguments: string;
	state: ToolCallState;
	approval?: { id: string; needsApproval: boolean; approved?: boolean };
	output?: unknown;
}

export interface ToolResultPart {
	type: 'tool-result';
	toolCallId: string;
	content: string;
	state: ToolResultState;
	error?: string;
}

export type MessagePart = TextPart | ThinkingPart | ToolCallPart | ToolResultPart;

export interface UIMessage {
	id: string;
	role: 'system' | 'user' | 'assistant';
	parts: MessagePart[];
	createdAt?: Date;
}

/** Multimodal-ready send payload (this app only sends text today). */
export interface MultimodalContent {
	content: string;
	id?: string;
}

export interface ChatRequestBody {
	messages: ModelMessage[];
	data?: Record<string, unknown>;
}

/** A connection yields the chunks for ONE assistant turn. */
export interface ConnectionAdapter {
	connect: (
		messages: UIMessage[],
		data?: Record<string, unknown>,
		abortSignal?: AbortSignal
	) => AsyncIterable<StreamChunk>;
}

/** A tool the client can execute or gate behind approval. */
export interface ClientTool {
	__toolSide: 'client';
	name: string;
	description: string;
	inputSchema?: unknown;
	outputSchema?: unknown;
	needsApproval?: boolean;
	metadata?: Record<string, unknown>;
	execute?: (args: unknown) => Promise<unknown> | unknown;
}

export type AnyClientTool = ClientTool;

export interface AddToolResultParams {
	toolCallId: string;
	tool: string;
	output: unknown;
	state?: 'output-available' | 'output-error';
	errorText?: string;
}

export interface CreateChatOptions {
	connection: ConnectionAdapter;
	id?: string;
	initialMessages?: UIMessage[];
	body?: Record<string, unknown>;
	tools?: ReadonlyArray<AnyClientTool>;
	onChunk?: (chunk: StreamChunk) => void;
	onFinish?: (message: UIMessage) => void;
	onError?: (error: Error) => void;
	onResponse?: () => void | Promise<void>;
}

export interface CreateChatReturn {
	readonly messages: UIMessage[];
	readonly isLoading: boolean;
	readonly error: Error | undefined;
	readonly status: ChatClientState;
	sendMessage: (content: string | MultimodalContent) => Promise<void>;
	append: (message: UIMessage) => Promise<void>;
	addToolResult: (result: AddToolResultParams) => Promise<void>;
	addToolApprovalResponse: (response: { id: string; approved: boolean }) => Promise<void>;
	reload: () => Promise<void>;
	stop: () => void;
	clear: () => void;
	setMessages: (messages: UIMessage[]) => void;
	updateBody: (body: Record<string, unknown>) => void;
}
