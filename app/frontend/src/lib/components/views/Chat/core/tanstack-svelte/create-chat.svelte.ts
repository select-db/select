import { ChatClient } from '@tanstack/ai-client';
import type { ChatClientState } from '@tanstack/ai-client';
import type { AnyClientTool, ModelMessage } from '@tanstack/ai';
import type { CreateChatOptions, CreateChatReturn, MultimodalContent, UIMessage } from './types';

type AddToolResultParam = Parameters<ChatClient['addToolResult']>[0];

/**
 * Creates a reactive chat instance for Svelte 5.
 * Wraps ChatClient from @tanstack/ai-client and exposes reactive state using Svelte 5 runes.
 */
export function createChat<TTools extends ReadonlyArray<AnyClientTool> = any>(
	options: CreateChatOptions<TTools>
): CreateChatReturn<TTools> {
	const clientId =
		options.id || `chat-${Date.now()}-${Math.random().toString(36).substring(7)}`;

	let messages = $state<UIMessage[]>(options.initialMessages || []);
	let isLoading = $state(false);
	let error = $state<Error | undefined>(undefined);
	let status = $state<ChatClientState>('ready');

	const client = new ChatClient({
		connection: options.connection,
		id: clientId,
		initialMessages: options.initialMessages,
		body: options.body,
		onResponse: options.onResponse,
		onChunk: options.onChunk,
		onFinish: (message) => {
			options.onFinish?.(message);
		},
		onError: (err) => {
			options.onError?.(err);
		},
		tools: options.tools,
		streamProcessor: options.streamProcessor,
		onMessagesChange: (newMessages: Array<UIMessage>) => {
			messages = newMessages;
		},
		onLoadingChange: (newIsLoading: boolean) => {
			isLoading = newIsLoading;
		},
		onStatusChange: (newStatus: ChatClientState) => {
			status = newStatus;
		},
		onErrorChange: (newError: Error | undefined) => {
			error = newError;
		}
	});

	const sendMessage = async (content: string | MultimodalContent) => {
		await client.sendMessage(content);
	};

	const append = async (message: ModelMessage | UIMessage) => {
		await client.append(message);
	};

	const reload = async () => {
		await client.reload();
	};

	const stop = () => {
		client.stop();
	};

	const clear = () => {
		client.clear();
	};

	const setMessages = (newMessages: Array<UIMessage>) => {
		client.setMessagesManually(newMessages);
	};

	const addToolResult = async (result: AddToolResultParam) => {
		await client.addToolResult(result);
	};

	const addToolApprovalResponse = async (response: { id: string; approved: boolean }) => {
		await client.addToolApprovalResponse(response);
	};

	const updateBody = (newBody: Record<string, unknown>) => {
		client.updateOptions({ body: newBody });
	};

	return {
		get messages() {
			return messages;
		},
		get isLoading() {
			return isLoading;
		},
		get error() {
			return error;
		},
		get status() {
			return status;
		},
		sendMessage,
		append,
		reload,
		stop,
		setMessages,
		clear,
		addToolResult,
		addToolApprovalResponse,
		updateBody
	};
}
