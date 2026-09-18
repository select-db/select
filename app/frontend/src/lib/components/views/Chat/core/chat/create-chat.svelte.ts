import { ChatClient } from './chat-client';
import type {
	AddToolResultParams,
	ChatClientState,
	CreateChatOptions,
	CreateChatReturn,
	MultimodalContent,
	UIMessage
} from './types';

/**
 * Reactive chat instance for Svelte 5. Wraps {@link ChatClient} and exposes its
 * state through runes so components re-render as messages stream in.
 */
export function createChat(options: CreateChatOptions): CreateChatReturn {
	let messages = $state<UIMessage[]>(options.initialMessages ?? []);
	let isLoading = $state(false);
	let error = $state<Error | undefined>(undefined);
	let status = $state<ChatClientState>('ready');

	const client = new ChatClient({
		connection: options.connection,
		id: options.id,
		initialMessages: options.initialMessages,
		body: options.body,
		tools: options.tools,
		onChunk: options.onChunk,
		onFinish: options.onFinish,
		onError: options.onError,
		onResponse: options.onResponse,
		onMessagesChange: (m) => (messages = m),
		onLoadingChange: (v) => (isLoading = v),
		onStatusChange: (s) => (status = s),
		onErrorChange: (e) => (error = e)
	});

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
		sendMessage: (content: string | MultimodalContent) => client.sendMessage(content),
		append: (message: UIMessage) => client.append(message),
		reload: () => client.reload(),
		stop: () => client.stop(),
		clear: () => client.clear(),
		setMessages: (m: UIMessage[]) => client.setMessagesManually(m),
		addToolResult: (result: AddToolResultParams) => client.addToolResult(result),
		addToolApprovalResponse: (response: { id: string; approved: boolean }) =>
			client.addToolApprovalResponse(response),
		updateBody: (body: Record<string, unknown>) => client.updateOptions({ body })
	};
}
