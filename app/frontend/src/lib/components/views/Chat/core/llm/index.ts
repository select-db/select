import type { ProviderStream } from './types';
import { streamAnthropic } from './anthropic';
import { streamGemini } from './gemini';
import { createOpenAICompatStream } from './openaiCompatible';

export type { StreamChunk, ModelMessage, ToolCall, LlmTool, StreamRequest, FinishReason } from './types';

/** OpenAI reasoning models (gpt-5.x, o-series) reject a custom temperature. */
const isOpenAiReasoningModel = (model: string) => /^(o\d|gpt-5)/i.test(model);

const streamOpenai = createOpenAICompatStream({
	baseURL: 'https://api.openai.com/v1',
	maxTokensField: 'max_completion_tokens',
	sendTemperature: (model) => !isOpenAiReasoningModel(model)
});

const streamGrok = createOpenAICompatStream({
	baseURL: 'https://api.x.ai/v1',
	maxTokensField: 'max_tokens'
});

const streamOpenRouter = createOpenAICompatStream({
	baseURL: 'https://openrouter.ai/api/v1'
});

const PROVIDERS: Record<string, ProviderStream> = {
	anthropic: streamAnthropic,
	gemini: streamGemini,
	openai: streamOpenai,
	grok: streamGrok,
	openrouter: streamOpenRouter
};

export function getProviderStream(provider: string): ProviderStream | undefined {
	return PROVIDERS[provider];
}
