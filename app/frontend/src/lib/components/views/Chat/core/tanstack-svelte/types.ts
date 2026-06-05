import type { AnyClientTool } from '@tanstack/ai';
import type { ChatClient } from '@tanstack/ai-client';
import type {
	ChatClientOptions,
	ChatClientState,
	ChatRequestBody,
	MultimodalContent,
	UIMessage,
	MessagePart,
	TextPart,
	ToolCallPart,
	ToolResultPart,
	ThinkingPart
} from '@tanstack/ai-client';

export type {
	ChatRequestBody,
	MultimodalContent,
	UIMessage,
	MessagePart,
	TextPart,
	ToolCallPart,
	ToolResultPart,
	ThinkingPart,
	ChatClientState
};

export type CreateChatOptions<TTools extends ReadonlyArray<AnyClientTool> = ReadonlyArray<AnyClientTool>> =
	Omit<
		ChatClientOptions<TTools>,
		'onMessagesChange' | 'onLoadingChange' | 'onErrorChange' | 'onStatusChange'
	>;

export type CreateChatReturn<TTools extends ReadonlyArray<AnyClientTool> = ReadonlyArray<AnyClientTool>> = {
	readonly messages: Array<UIMessage<TTools>>;
	readonly isLoading: boolean;
	readonly error: Error | undefined;
	readonly status: ChatClientState;
	sendMessage: ChatClient['sendMessage'];
	append: ChatClient['append'];
	addToolResult: ChatClient['addToolResult'];
	addToolApprovalResponse: ChatClient['addToolApprovalResponse'];
	reload: () => Promise<void>;
	stop: () => void;
	clear: () => void;
	setMessages: (messages: Array<UIMessage<TTools>>) => void;
	updateBody: (body: Record<string, unknown>) => void;
};
