import type { UIMessage } from '$lib/components/views/Chat/core';
import { SYSTEM_PROMPT } from './systemPrompt';

export const API_KEY_ENV: Record<string, string> = {
	openai: 'OPENAI_API_KEY',
	anthropic: 'ANTHROPIC_API_KEY',
	gemini: 'GEMINI_API_KEY',
	openrouter: 'OPENROUTER_API_KEY',
	grok: 'XAI_API_KEY'
};

export const DEFAULT_MODEL = 'anthropic:claude-sonnet-4-5';

/** User message sent when opening chat with a task; hidden in UI. Tells the model to follow the <task> in the system message. */
export const TASK_TRIGGER_MESSAGE =
	'Please follow the instruction in the <task> block from the system message.';

/** Optional append (e.g. <task> block) is added after the system prompt and not shown to the user. */
export function createSystemMessage(append?: string): UIMessage {
	const content = append ? `${SYSTEM_PROMPT}\n\n${append}` : SYSTEM_PROMPT;
	return {
		id: 'system',
		role: 'system',
		parts: [{ type: 'text', content }]
	};
}
