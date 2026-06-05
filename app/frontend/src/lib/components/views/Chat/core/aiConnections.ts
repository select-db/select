import type { AnyClientTool } from '@tanstack/ai';
import { chat, convertMessagesToModelMessages } from '@tanstack/ai';
import { createOpenaiChat, OPENAI_CHAT_MODELS } from '@tanstack/ai-openai';
import {
	createAnthropicChat,
	ANTHROPIC_MODELS as ANTHROPIC_CHAT_MODELS
} from '@tanstack/ai-anthropic';
import { createGeminiChat, GeminiTextModels as GEMINI_CHAT_MODELS } from '@tanstack/ai-gemini';
import { createOpenRouterText } from '@tanstack/ai-openrouter';
import { createGrokText, GROK_CHAT_MODELS } from '@tanstack/ai-grok';
import type { ConnectionAdapter, UIMessage } from './tanstack-svelte';
import type { SelectOptionGroup } from '$lib/system/Select/Select.types';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { get } from 'svelte/store';
import * as graphApi from '$lib/wailsjs/go/graph/Graph';
import { tryCatch } from '$lib/utils/tryCatch';
import type { graph } from '$lib/wailsjs/go/models';

// Curated for high TPM/throughput (agent use). Model IDs must exist in @tanstack/ai-openai, ai-anthropic, ai-gemini, ai-grok.
// OpenRouter accepts any model ID string. Order: higher throughput first.
const OPENAI_POPULAR_CHAT_MODELS = [
	'gpt-4o-mini', // highest TPM tier
	'gpt-4o',
	'gpt-5-mini',
	'gpt-5.2'
];
const ANTHROPIC_POPULAR = [
	'claude-sonnet-4-5', // high TPM, good for agents
	'claude-haiku-4-5',
	'claude-opus-4-5',
	'claude-sonnet-4'
];
const GEMINI_POPULAR = [
	'gemini-2.5-flash', // 250k+ TPM
	'gemini-2.5-pro',
	'gemini-2.0-flash'
];
const OPENROUTER_POPULAR = [
	'openai/gpt-4o-mini',
	'openai/gpt-4o',
	'anthropic/claude-sonnet-4',
	'google/gemini-2.5-flash',
	'meta-llama/llama-3.3-70b-instruct'
];
const GROK_POPULAR = ['grok-4-1-fast-reasoning', 'grok-4'];

const SEP = ':';

const toGroup = (
	label: string,
	modelIds: readonly string[],
	provider: string
): SelectOptionGroup<string> => ({
	label,
	options: modelIds.map((id) => ({ value: `${provider}${SEP}${id}`, label: id }))
});

/** Most popular text (chat) models per provider, grouped. Value format: "provider:modelId". Ollama excluded (Node-only, not bundleable for browser). */
export const AI_MODEL_OPTION_GROUPS: SelectOptionGroup<string>[] = [
	toGroup('Anthropic Claude', ANTHROPIC_POPULAR, 'anthropic'),
	toGroup('OpenAI Chat', OPENAI_POPULAR_CHAT_MODELS, 'openai'),
	toGroup('Google Gemini', GEMINI_POPULAR, 'gemini'),
	toGroup('OpenRouter', OPENROUTER_POPULAR, 'openrouter'),
	toGroup('Grok (xAI)', GROK_POPULAR, 'grok')
];

export function getProvider(value: string): string {
	const i = value.indexOf(SEP);
	return i > 0 ? value.slice(0, i) : 'openai';
}

export function getModelId(value: string): string {
	const i = value.indexOf(SEP);
	return i > 0 ? value.slice(i + 1) : value;
}

const PROVIDER_ENV_VARS: Record<string, string> = {
	openai: 'OPENAI_API_KEY',
	anthropic: 'ANTHROPIC_API_KEY',
	gemini: 'GEMINI_API_KEY',
	openrouter: 'OPENROUTER_API_KEY',
	grok: 'XAI_API_KEY'
};

let cachedRootFolderId: string | null = null;
let cachedVars: Record<string, string> | null = null;

async function getWorkspaceEnvVars(): Promise<Record<string, string>> {
	const ws = get(workspaceGraphStore) as graph.WorkspaceNode | undefined;
	const rootFolderId = ws?.folders?.[0]?.id ?? '';
	if (!rootFolderId) return {};

	if (cachedRootFolderId === rootFolderId && cachedVars) {
		return cachedVars;
	}

	const [vars, err] = await tryCatch(graphApi.GetUriVariables, rootFolderId);
	if (err || !vars) {
		cachedRootFolderId = rootFolderId;
		cachedVars = {};
		return cachedVars;
	}

	const map: Record<string, string> = {};
	for (const v of vars as Array<{ name?: string; value?: string }>) {
		if (typeof v?.name === 'string' && typeof v?.value === 'string') {
			map[v.name] = v.value;
		}
	}

	cachedRootFolderId = rootFolderId;
	cachedVars = map;
	return map;
}

async function getProviderApiKey(provider: string): Promise<string> {
	const envVar = PROVIDER_ENV_VARS[provider];
	if (!envVar) return '';
	const vars = await getWorkspaceEnvVars();
	return vars[envVar] ?? '';
}

export async function hasApiKey(provider: string): Promise<boolean> {
	const key = await getProviderApiKey(provider);
	return key.length > 0;
}

/** True if workspace root .env defines at least one known LLM API key (non-empty). */
export async function hasAnyWorkspaceApiKey(): Promise<boolean> {
	const vars = await getWorkspaceEnvVars();
	for (const envName of Object.values(PROVIDER_ENV_VARS)) {
		const v = vars[envName]?.trim();
		if (v && v.length > 0) return true;
	}
	return false;
}

/** First model value (provider:modelId) whose provider has an API key set in workspace env. Use to preselect model on chat mount. */
export async function getPreferredModelWithApiKey(): Promise<string> {
	for (const group of AI_MODEL_OPTION_GROUPS) {
		const first = group.options[0];
		if (!first?.value) continue;
		const provider = getProvider(first.value);
		if (await hasApiKey(provider)) return first.value;
	}
	return 'anthropic:claude-sonnet-4-5';
}

function toErrorMessage(err: unknown): string {
	if (err instanceof Error) {
		const msg = err.message;
		const jsonMatch = msg.match(/\{[\s\S]*"error"[\s\S]*"message"[\s\S]*\}/);
		if (jsonMatch) {
			try {
				const data = JSON.parse(jsonMatch[0]) as { error?: { message?: string }; message?: string };
				const extracted = data?.error?.message ?? data?.message;
				if (typeof extracted === 'string' && extracted.length > 0) return extracted;
			} catch {
				// use raw message
			}
		}
		return msg.length > 400 ? msg.slice(0, 400) + '…' : msg;
	}
	return String(err);
}

type MessageLike = {
	role: string;
	parts?: Array<{ type?: string; content?: unknown }>;
};

/**
 * Anthropic is strict about trailing whitespace on final assistant content.
 * To be safe across providers, trim trailing whitespace from all assistant text parts.
 */
function trimAssistantTrailingWhitespace(messages: unknown[]): unknown[] {
	const msgs = messages as MessageLike[];
	return msgs.map((msg) => {
		if (msg.role !== 'assistant' || !msg.parts || msg.parts.length === 0) return msg;

		const trimmedParts = msg.parts.map((part) => {
			const p = part as { type?: string; content?: unknown };
			if (p.type === 'text' && typeof p.content === 'string') {
				return { ...part, content: p.content.replace(/\s+$/u, '') };
			}
			return part;
		});

		return {
			...msg,
			parts: trimmedParts
		};
	});
}

/**
 * Applies a sliding window to keep the most recent N messages.
 * Always keeps the system message (first message with role 'system').
 * Keeps the most recent non-system messages up to maxMessages limit.
 */
function applySlidingWindow(messages: unknown[], maxMessages: number = 30): unknown[] {
	const msgs = messages as MessageLike[];
	if (messages.length <= maxMessages) return messages;

	// Find system message (usually first, but search to be safe)
	const systemMsg = msgs.find((m) => m.role === 'system');
	const nonSystemMessages = msgs.filter((m) => m.role !== 'system');

	// Keep most recent non-system messages
	const recentMessages = nonSystemMessages.slice(-maxMessages + (systemMsg ? 1 : 0));

	// Combine: system message first (if exists), then recent messages
	return systemMsg ? [systemMsg, ...recentMessages] : recentMessages;
}

/**
 * Prepares messages for LLM by normalizing content and applying sliding window.
 */
function prepareMessagesForLLM(messages: unknown[]): unknown[] {
	const trimmed = trimAssistantTrailingWhitespace(messages);
	return applySlidingWindow(trimmed);
}

/**
 * Ensures every assistant tool_use has a corresponding tool_result in the next message(s).
 * Anthropic (and others) require this; otherwise they return 400. When multiple tools are
 * invoked in one turn, one can stay pending (e.g. plan_query stuck) so we inject a synthetic
 * result for any missing ID to avoid invalid_request_error.
 */
type ModelMessageLike = {
	role: 'user' | 'assistant' | 'tool';
	content?: string | null | unknown[];
	toolCalls?: Array<{
		id: string;
		type: 'function';
		function: { name: string; arguments: string };
	}>;
	toolCallId?: string;
};

function ensureToolResultsForAllToolCalls(messages: ModelMessageLike[]): ModelMessageLike[] {
	const out: ModelMessageLike[] = [];
	for (let i = 0; i < messages.length; i++) {
		const msg = messages[i];
		if (msg.role !== 'assistant' || !msg.toolCalls?.length) {
			out.push(msg);
			continue;
		}
		out.push(msg);
		const requiredIds = new Set(msg.toolCalls.map((tc) => tc.id));
		let j = i + 1;
		while (j < messages.length && messages[j].role === 'tool') {
			const toolMsg = messages[j];
			const toolCallId = toolMsg.toolCallId;
			if (toolCallId) {
				out.push(toolMsg);
				requiredIds.delete(toolCallId);
			}
			j++;
		}
		for (const id of requiredIds) {
			out.push({
				role: 'tool',
				content: JSON.stringify({
					error: 'Tool execution did not complete.',
					success: false
				}),
				toolCallId: id
			});
		}
		i = j - 1;
	}
	return out;
}

/**
 * Provider-specific options passed to `chat({ modelOptions })` so reasoning/thinking
 * stays in API-internal channels where supported, and does not leak into visible text.
 */
function modelOptionsForProvider(provider: string, modelId: string): Record<string, unknown> {
	switch (provider) {
		case 'anthropic':
			return { thinking: { type: 'disabled' } };
		case 'openrouter':
			return { reasoning: { exclude: true } };
		case 'openai':
			// OpenAI Responses API: `none` is supported on gpt-5.x; older models ignore or reject, only set for that family.
			if (/^gpt-5/i.test(modelId)) {
				return { reasoning: { effort: 'none' } };
			}
			return {};
		case 'gemini':
			// 2.5+ models support thinking; omit on 2.0 etc. to avoid API errors.
			if (/^gemini-2\.5/i.test(modelId)) {
				return { thinkingConfig: { includeThoughts: false } };
			}
			return {};
		case 'grok':
			// Grok uses Chat Completions; pick a *-non-reasoning model in the UI for less reasoning output.
			return {};
		default:
			return {};
	}
}

export function createUnifiedConnection(tools?: ReadonlyArray<AnyClientTool>): ConnectionAdapter {
	return {
		async *connect(messages, data, abortSignal) {
			if (!messages.length) return;
			const provider = (data?.provider as string) ?? 'anthropic';
			const model = (data?.model as string) ?? 'claude-sonnet-4-5';

			const key = await getProviderApiKey(provider);
			if (!key.length) {
				const envVars: Record<string, string> = {
					openai: PROVIDER_ENV_VARS.openai,
					anthropic: PROVIDER_ENV_VARS.anthropic,
					gemini: PROVIDER_ENV_VARS.gemini,
					openrouter: PROVIDER_ENV_VARS.openrouter,
					grok: PROVIDER_ENV_VARS.grok
				};
				throw new Error(`Set ${envVars[provider] ?? 'API key'} in .env for ${provider}.`);
			}

			let adapter: Parameters<typeof chat>[0]['adapter'];
			switch (provider) {
				case 'openai':
					adapter = createOpenaiChat(model as (typeof OPENAI_CHAT_MODELS)[number], key, {
						dangerouslyAllowBrowser: true
					});
					break;
				case 'anthropic':
					adapter = createAnthropicChat(model as (typeof ANTHROPIC_CHAT_MODELS)[number], key, {
						dangerouslyAllowBrowser: true
					});
					break;
				case 'gemini':
					adapter = createGeminiChat(model as (typeof GEMINI_CHAT_MODELS)[number], key);
					break;
				case 'openrouter':
					adapter = createOpenRouterText(model as Parameters<typeof createOpenRouterText>[0], key);
					break;
				case 'grok':
					adapter = createGrokText(model as (typeof GROK_CHAT_MODELS)[number], key, {
						dangerouslyAllowBrowser: true
					});
					break;
				default:
					throw new Error(`Unknown provider: ${provider}`);
			}

			// Prepare messages: filter tool parts and apply sliding window
			const preparedMessages = prepareMessagesForLLM(messages);
			const systemMsg = (preparedMessages as MessageLike[]).find((m) => m.role === 'system');
			const systemContent =
				typeof systemMsg?.parts?.[0] === 'object' &&
				systemMsg.parts[0] != null &&
				'content' in systemMsg.parts[0]
					? String(systemMsg.parts[0].content)
					: '';

			const systemPrompts = systemContent ? [systemContent] : undefined;

			const modelOptions = modelOptionsForProvider(provider, model);

			const controller = abortSignal
				? (() => {
						const c = new AbortController();
						abortSignal.addEventListener('abort', () => c.abort(), { once: true });
						return c;
					})()
				: undefined;
			try {
				const modelMessages = convertMessagesToModelMessages(
					preparedMessages as unknown as UIMessage[]
				) as ModelMessageLike[];
				const messagesWithToolResults = ensureToolResultsForAllToolCalls(modelMessages);
				const stream = chat({
					adapter,
					messages: messagesWithToolResults,
					systemPrompts,
					modelOptions,
					tools,
					abortController: controller,
					temperature: 0
				} as Parameters<typeof chat>[0]);
				if (Symbol.asyncIterator in Object(stream)) {
					yield* stream as AsyncIterable<import('@tanstack/ai').StreamChunk>;
				}
			} catch (err) {
				throw new Error(toErrorMessage(err));
			}
		}
	};
}
