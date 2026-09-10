import { expect, type Page } from './wails';

/**
 * The AI providers, answered from here. Nothing reaches a real API: no network,
 * no cost, and the same reply every run. The app still wants a key present
 * before it tries, and reads it from the workspace .env where the seed leaves a
 * placeholder for each provider.
 *
 * A spec describes a turn once and each provider renders it in its own wire
 * format, which is the only thing that differs between them: the app has to end
 * every call the model makes, whichever provider carried it.
 *
 * The two controls for talking to a chat live here too, since every spec that
 * scripts a provider also has to pick one and type at it.
 */

/** One assistant turn, before any provider has had a say in how it looks. */
export type Turn = {
	text?: string;
	call?: { name: string; input: object; id: string };
	/**
	 * End the turn with the error a provider sends when it is overloaded, after
	 * the tool call is already complete. The call is good; only the stream broke.
	 */
	overloaded?: boolean;
	/**
	 * Stop partway through the tool arguments, leaving them as JSON that never
	 * closes. Only the providers that stream arguments in fragments can do this;
	 * Gemini hands over an already-parsed object and refuses the request.
	 */
	truncated?: boolean;
};

export type AiProvider = {
	/** What the model picker shows for it. */
	model: string;
	/** Which requests to answer. */
	url: string;
	/** Renders one turn into this provider's wire format. */
	render: (turn: Turn) => string;
};

const OVERLOADED = 'Overloaded';

/**
 * One frame of a Server-Sent Events stream, which is how all three formats
 * arrive: an optional `event:` name, then the payload on a `data:` line.
 */
const sseFrame = (payload: object, eventName?: string) =>
	`${eventName ? `event: ${eventName}\n` : ''}data: ${JSON.stringify(payload)}\n\n`;

/**
 * A tool call's arguments as a provider sends them: the JSON cut into small
 * pieces, never in one go. A call with no input sends nothing at all, which is
 * what leaves the app holding an empty string rather than `{}`.
 */
const streamedArgs = (input: object, truncated = false) => {
	const json = JSON.stringify(input);
	if (json === '{}') return [];
	const pieces = json.match(/.{1,7}/g) ?? [];
	return truncated ? pieces.slice(0, 1) : pieces;
};

/** Anthropic streams typed events, one per content block, around a message. */
const renderAnthropic = (turn: Turn): string => {
	const event = (type: string, data: object) => sseFrame({ type, ...data }, type);
	let index = 0;
	let out = sseFrame({
		type: 'message_start',
		message: { id: 'msg_1', role: 'assistant', content: [] }
	});

	if (turn.text) {
		out +=
			event('content_block_start', { index, content_block: { type: 'text', text: '' } }) +
			event('content_block_delta', { index, delta: { type: 'text_delta', text: turn.text } }) +
			event('content_block_stop', { index });
		index += 1;
	}
	if (turn.call) {
		out +=
			event('ping', {}) +
			event('content_block_start', {
				index,
				content_block: { type: 'tool_use', id: turn.call.id, name: turn.call.name, input: {} }
			}) +
			streamedArgs(turn.call.input, turn.truncated)
				.map((piece) =>
					event('content_block_delta', {
						index,
						delta: { type: 'input_json_delta', partial_json: piece }
					})
				)
				.join('');
		if (turn.truncated) return out;
		out += event('content_block_stop', { index });
	}

	if (turn.overloaded)
		return out + event('error', { error: { type: 'overloaded_error', message: OVERLOADED } });

	return (
		out +
		event('message_delta', { delta: { stop_reason: turn.call ? 'tool_use' : 'end_turn' } }) +
		event('message_stop', {})
	);
};

/**
 * The Chat Completions wire format, shared by OpenAI, xAI Grok and OpenRouter.
 * It streams chunks, each carrying the delta to add to the message so far.
 */
const renderChatCompletions = (turn: Turn): string => {
	const chunk = (delta: object) => sseFrame({ choices: [{ index: 0, delta }] });
	let out = '';

	if (turn.text) out += chunk({ content: turn.text });
	if (turn.call) {
		out += chunk({
			tool_calls: [
				{
					index: 0,
					id: turn.call.id,
					type: 'function',
					function: { name: turn.call.name, arguments: '' }
				}
			]
		});
		out += streamedArgs(turn.call.input, turn.truncated)
			.map((piece) => chunk({ tool_calls: [{ index: 0, function: { arguments: piece } }] }))
			.join('');
		if (turn.truncated) return out;
	}

	if (turn.overloaded) return out + sseFrame({ error: { message: OVERLOADED } });

	return (
		out +
		sseFrame({
			choices: [{ index: 0, delta: {}, finish_reason: turn.call ? 'tool_calls' : 'stop' }]
		}) +
		'data: [DONE]\n\n'
	);
};

/**
 * Gemini names no call ids on the wire and matches results back by function
 * name, so the ids the app runs on are its own. That is why the case worth
 * running here is a conversation with more than one call.
 */
const renderGemini = (turn: Turn): string => {
	if (turn.truncated) {
		throw new Error('Gemini sends tool arguments already parsed; it cannot cut them short.');
	}
	const parts = [
		...(turn.text ? [{ text: turn.text }] : []),
		...(turn.call ? [{ functionCall: { name: turn.call.name, args: turn.call.input } }] : [])
	];
	const out = sseFrame({ candidates: [{ content: { parts }, finishReason: 'STOP' }] });
	return turn.overloaded ? out + sseFrame({ error: { message: OVERLOADED } }) : out;
};

/**
 * Every provider the app offers. The model is the first of each group in
 * aiConnections, which is the one a person is given by default.
 */
export const PROVIDERS: AiProvider[] = [
	{ model: 'claude-sonnet-5', url: 'https://api.anthropic.com/**', render: renderAnthropic },
	{ model: 'gpt-5.6-sol', url: 'https://api.openai.com/**', render: renderChatCompletions },
	{
		model: 'gemini-3.7-flash',
		url: 'https://generativelanguage.googleapis.com/**',
		render: renderGemini
	},
	{
		model: 'anthropic/claude-sonnet-5',
		url: 'https://openrouter.ai/**',
		render: renderChatCompletions
	},
	{ model: 'grok-4.6', url: 'https://api.x.ai/**', render: renderChatCompletions }
];

/**
 * The provider a fresh chat starts on: the first option whose provider has a
 * key, and so the one a spec gets without asking for it.
 */
export const ANTHROPIC = PROVIDERS[0];

/**
 * Sets up what the model will answer, so nothing leaves the machine: the app's
 * requests to this provider are intercepted, and each one is served the next
 * turn in the provider's own wire format. Once the turns run out the last one
 * is repeated, since the app keeps asking until a turn makes no tool call.
 */
export async function modelWillReply(page: Page, provider: AiProvider, turns: Turn[]) {
	// Rendered up front, so a turn this provider cannot express fails here rather
	// than inside a route handler, where it would surface as a stalled request.
	const replies = turns.map(provider.render);
	let sent = 0;
	await page.route(provider.url, async (route) => {
		const body = replies[Math.min(sent, replies.length - 1)];
		sent += 1;
		await route.fulfill({
			status: 200,
			headers: { 'content-type': 'text/event-stream' },
			body
		});
	});
}

/** The box a person types into, and the only thing naming it. */
export const prompt = (page: Page) => page.getByRole('textbox', { name: 'Type a message...' });

/** Types a message into the chat and sends it. */
export async function say(page: Page, message: string) {
	const box = prompt(page);
	await expect(box).toBeVisible();
	await box.click();
	await page.keyboard.type(message);
	await page.keyboard.press('Enter');
}

/**
 * Moves the model picker, addressed by the model it is showing: that is its
 * accessible name, and the only thing a person could call it by.
 */
export async function chooseModel(page: Page, model: string) {
	await expect(page.getByRole('button', { name: ANTHROPIC.model })).toBeVisible();
	if (model === ANTHROPIC.model) return;

	await page.getByRole('button', { name: ANTHROPIC.model }).click();
	await page.getByRole('menuitem', { name: model, exact: true }).click();
	await expect(page.getByRole('button', { name: model })).toBeVisible();
}
