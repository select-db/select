export {
	createChat,
	fetchServerSentEvents,
	fetchHttpStream,
	stream,
	createChatClientOptions,
	clientTools
} from './tanstack-svelte';

export type {
	CreateChatOptions,
	CreateChatReturn,
	UIMessage,
	MessagePart,
	ToolCallPart,
	ChatRequestBody,
	ConnectionAdapter,
	FetchConnectionOptions,
	InferChatMessages
} from './tanstack-svelte';
