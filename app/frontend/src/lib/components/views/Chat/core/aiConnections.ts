import type { ConnectionAdapter, UIMessage, AnyClientTool, ModelMessage } from './chat';
import { convertMessagesToModelMessages, toParametersJsonSchema } from './chat';
import { getProviderStream } from './llm';
import type { LlmTool, StreamChunk } from './llm';
import type { SelectOptionGroup } from '$lib/system/Select/Select.types';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import { get } from 'svelte/store';
import type { graph } from '$lib/wailsjs/go/models';

// Current, agent-suitable models per provider (tool-calling required). First entry
// in each group is the sensible default. OpenRouter accepts any model ID string.
const ANTHROPIC_POPULAR = [
	'claude-sonnet-5', // SOTA balance, 1M context — default
	'claude-haiku-4-5', // fastest & cheapest, great for tool-heavy loops
	'claude-opus-5' // most capable
];
const OPENAI_POPULAR_CHAT_MODELS = [
	'gpt-5.6-sol', // flagship
	'gpt-5.6-terra', // balanced
	'gpt-5.6-luna' // fast & cheap
];
const GEMINI_POPULAR = [
	'gemini-3.7-flash', // fast & cheap
	'gemini-3.1-pro-preview', // most capable
	'gemini-3.5-flash-lite' // cheapest
];
const OPENROUTER_POPULAR = [
	'anthropic/claude-sonnet-5',
	'openai/gpt-5.6-sol',
	'google/gemini-3.7-flash',
	'x-ai/grok-4.6'
];
const GROK_POPULAR = ['grok-4.6', 'grok-4.20'];

const SEP = ':';

const toGroup = (
	label: string,
	modelIds: readonly string[],
	provider: string
): SelectOptionGroup<string> => ({
	label,
	options: modelIds.map((id) => ({ value: `${provider}${SEP}${id}`, label: id }))
});

/** Most popular text (chat) models per provider, grouped. Value format: "provider:modelId". */
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

// Read env vars straight from the reactive workspace graph store. The backend
// file watcher updates the root folder's `variables` and emits
// `workspaceGraphUpdated` whenever a .env file changes, so this stays live
// without a refresh. The chat only ever needs the root folder, and
// GetUriVariables(rootFolderId) resolves to exactly the root folder's own
// variables (it walks up the tree, and the root has no parent), so reading the
// store directly is equivalent — no RPC or cache needed.
function getWorkspaceEnvVars(): Record<string, string> {
	const ws = get(workspaceGraphStore) as graph.WorkspaceNode | undefined;
	return ws?.folders?.[0]?.variables ?? {};
}

async function getProviderApiKey(provider: string): Promise<string> {
	const envVar = PROVIDER_ENV_VARS[provider];
	if (!envVar) return '';
	const vars = getWorkspaceEnvVars();
	return vars[envVar] ?? '';
}

export async function hasApiKey(provider: string): Promise<boolean> {
	const key = await getProviderApiKey(provider);
	return key.length > 0;
}

/** True if workspace root .env defines at least one known LLM API key (non-empty). */
export async function hasAnyWorkspaceApiKey(): Promise<boolean> {
	const vars = getWorkspaceEnvVars();
	for (const envName of Object.values(PROVIDER_ENV_VARS)) {
		const v = vars[envName]?.trim();
		if (v && v.length > 0) return true;
	}
	return false;
}

/** First model value (provider:modelId) whose provider has an API key set in workspace env. */
export async function getPreferredModelWithApiKey(): Promise<string> {
	for (const group of AI_MODEL_OPTION_GROUPS) {
		const first = group.options[0];
		if (!first?.value) continue;
		const provider = getProvider(first.value);
		if (await hasApiKey(provider)) return first.value;
	}
	return 'anthropic:claude-sonnet-5';
}

type MessageLike = {
	role: string;
	parts?: Array<{ type?: string; content?: unknown }>;
};

/**
 * Anthropic is strict about trailing whitespace on final assistant content.
 * To be safe across providers, trim trailing whitespace from all assistant text parts.
 */
function trimAssistantTrailingWhitespace(messages: UIMessage[]): UIMessage[] {
	const msgs = messages as unknown as MessageLike[];
	return msgs.map((msg) => {
		if (msg.role !== 'assistant' || !msg.parts || msg.parts.length === 0)
			return msg as unknown as UIMessage;

		const trimmedParts = msg.parts.map((part) => {
			const p = part as { type?: string; content?: unknown };
			if (p.type === 'text' && typeof p.content === 'string') {
				return { ...part, content: p.content.replace(/\s+$/u, '') };
			}
			return part;
		});

		return { ...msg, parts: trimmedParts } as unknown as UIMessage;
	});
}

/**
 * Applies a sliding window to keep the most recent N messages.
 * Always keeps the system message (first message with role 'system').
 */
function applySlidingWindow(messages: UIMessage[], maxMessages = 30): UIMessage[] {
	if (messages.length <= maxMessages) return messages;

	const systemMsg = messages.find((m) => m.role === 'system');
	const nonSystemMessages = messages.filter((m) => m.role !== 'system');
	const recentMessages = nonSystemMessages.slice(-maxMessages + (systemMsg ? 1 : 0));

	return systemMsg ? [systemMsg, ...recentMessages] : recentMessages;
}

function prepareMessagesForLLM(messages: UIMessage[]): UIMessage[] {
	return applySlidingWindow(trimAssistantTrailingWhitespace(messages));
}

/**
 * Ensures every assistant tool_use has a corresponding tool_result. Anthropic (and
 * others) require this; when multiple tools are invoked in one turn one can stay
 * pending, so we inject a synthetic result for any missing ID to avoid a 400.
 */
function ensureToolResultsForAllToolCalls(messages: ModelMessage[]): ModelMessage[] {
	const out: ModelMessage[] = [];
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
			if (toolMsg.toolCallId) {
				out.push(toolMsg);
				requiredIds.delete(toolMsg.toolCallId);
			}
			j++;
		}
		for (const id of requiredIds) {
			out.push({
				role: 'tool',
				content: JSON.stringify({ error: 'Tool execution did not complete.', success: false }),
				toolCallId: id
			});
		}
		i = j - 1;
	}
	return out;
}

/**
 * Provider-specific options merged into the request body so reasoning/thinking
 * stays in API-internal channels and does not leak into visible text.
 */
function modelOptionsForProvider(provider: string, modelId: string): Record<string, unknown> {
	switch (provider) {
		case 'openrouter':
			return { reasoning: { exclude: true } };
		case 'gemini':
			// 2.5+ and 3.x support thinking; hide the thoughts so they don't leak into text.
			if (/^gemini-([2-9]\.5|[3-9])/i.test(modelId)) {
				return { thinkingConfig: { includeThoughts: false } };
			}
			return {};
		default:
			// OpenAI/Grok Chat Completions never stream reasoning as content; Anthropic
			// leaves extended thinking off unless explicitly enabled. Nothing to send.
			return {};
	}
}

function toLlmTools(tools?: ReadonlyArray<AnyClientTool>): LlmTool[] | undefined {
	if (!tools?.length) return undefined;
	return tools.map((t) => ({
		name: t.name,
		description: t.description,
		parameters: toParametersJsonSchema(t.inputSchema)
	}));
}

function extractSystemPrompt(messages: UIMessage[]): string | undefined {
	const systemMsg = messages.find((m) => m.role === 'system');
	const first = systemMsg?.parts?.[0];
	if (first && first.type === 'text' && typeof first.content === 'string') return first.content;
	return undefined;
}

/** Build a connection that streams one assistant turn from the selected provider. */
export function createUnifiedConnection(tools?: ReadonlyArray<AnyClientTool>): ConnectionAdapter {
	const llmTools = toLlmTools(tools);

	return {
		async *connect(messages, data, abortSignal): AsyncIterable<StreamChunk> {
			if (!messages.length) return;
			const provider = (data?.provider as string) ?? 'anthropic';
			const model = (data?.model as string) ?? 'claude-sonnet-5';

			const key = await getProviderApiKey(provider);
			if (!key.length) {
				throw new Error(`Set ${PROVIDER_ENV_VARS[provider] ?? 'API key'} in .env for ${provider}.`);
			}

			const stream = getProviderStream(provider);
			if (!stream) throw new Error(`Unknown provider: ${provider}`);

			const prepared = prepareMessagesForLLM(messages);
			const system = extractSystemPrompt(prepared);
			const modelMessages = ensureToolResultsForAllToolCalls(
				convertMessagesToModelMessages(prepared)
			);

			yield* stream({
				model,
				apiKey: key,
				system,
				messages: modelMessages,
				tools: llmTools,
				temperature: 0,
				modelOptions: modelOptionsForProvider(provider, model),
				signal: abortSignal
			});
		}
	};
}
