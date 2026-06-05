export { createChat } from './create-chat.svelte';
export type {
	CreateChatOptions,
	CreateChatReturn,
	UIMessage,
	MessagePart,
	ToolCallPart,
	ChatRequestBody
} from './types';

export {
	fetchServerSentEvents,
	fetchHttpStream,
	stream,
	createChatClientOptions,
	clientTools,
	type ConnectionAdapter,
	type FetchConnectionOptions,
	type InferChatMessages
} from '@tanstack/ai-client';
