import { expect, type Page } from './wails';

/**
 * The AI providers, answered from here. Nothing reaches a real API: no network,
 * no cost, and the same reply every run. The app still wants a key present
 * before it tries, and reads it from the workspace .env where the seed leaves a
 * placeholder for each provider.
 *
 * A spec describes a reply once and each provider renders it in its own wire
 * format, which is the only thing that differs between them: the app has to end
 * every call the model makes, whichever provider carried it.
 *
 * The two controls for talking to a chat live here too, since every spec that
 * scripts a provider also has to pick one and type at it.
 */

/**
 * One reply from the model: what it says, and the tool it asks the app to run.
 * Written once per spec, before any provider has had a say in how it looks.
 */
export type ModelReply = {
	text?: string;
	call?: { name: string; input: object; id: string };
	/**
	 * End with the error a provider sends when it is overloaded, after the tool
	 * call is already complete. The call is good; only the stream broke.
	 */
	overloaded?: boolean;
	/**
	 * Stop partway through the tool arguments, leaving them as JSON that never
	 * closes. Only the providers that stream arguments in fragments can do this;
	 * Gemini hands over an already-parsed object and refuses the request.
	 */
	truncated?: boolean;
};

/**
 * A whole HTTP response body, in the Server-Sent Events format all three wire
 * formats stream over: a run of `data:` lines the app reads as it arrives.
 */
type SseStream = string;

export type AiProvider = {
	/** What the model picker shows for it. */
	model: string;
	/** Which requests to answer. */
	url: string;
	/** Renders one reply into this provider's wire format. */
	render: (reply: ModelReply) => SseStream;
};

const OVERLOADED = 'Overloaded';

/** One frame: an optional `event:` name, then the payload on a `data:` line. */
const sseFrame = (payload: object, eventName?: string): SseStream =>
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
const renderAnthropic = (reply: ModelReply): SseStream => {
	const event = (type: string, data: object) => sseFrame({ type, ...data }, type);
	let index = 0;
	let out = sseFrame({
		type: 'message_start',
		message: { id: 'msg_1', role: 'assistant', content: [] }
	});

	if (reply.text) {
		out +=
			event('content_block_start', { index, content_block: { type: 'text', text: '' } }) +
			event('content_block_delta', { index, delta: { type: 'text_delta', text: reply.text } }) +
			event('content_block_stop', { index });
		index += 1;
	}
	if (reply.call) {
		out +=
			event('ping', {}) +
			event('content_block_start', {
				index,
				content_block: { type: 'tool_use', id: reply.call.id, name: reply.call.name, input: {} }
			}) +
			streamedArgs(reply.call.input, reply.truncated)
				.map((piece) =>
					event('content_block_delta', {
						index,
						delta: { type: 'input_json_delta', partial_json: piece }
					})
				)
				.join('');
		if (reply.truncated) return out;
		out += event('content_block_stop', { index });
	}

	if (reply.overloaded)
		return out + event('error', { error: { type: 'overloaded_error', message: OVERLOADED } });

	return (
		out +
		event('message_delta', { delta: { stop_reason: reply.call ? 'tool_use' : 'end_turn' } }) +
		event('message_stop', {})
	);
};

/**
 * The Chat Completions wire format, shared by OpenAI, xAI Grok and OpenRouter.
 * It streams chunks, each carrying the delta to add to the message so far.
 */
const renderChatCompletions = (reply: ModelReply): SseStream => {
	const chunk = (delta: object) => sseFrame({ choices: [{ index: 0, delta }] });
	let out = '';

	if (reply.text) out += chunk({ content: reply.text });
	if (reply.call) {
		out += chunk({
			tool_calls: [
				{
					index: 0,
					id: reply.call.id,
					type: 'function',
					function: { name: reply.call.name, arguments: '' }
				}
			]
		});
		out += streamedArgs(reply.call.input, reply.truncated)
			.map((piece) => chunk({ tool_calls: [{ index: 0, function: { arguments: piece } }] }))
			.join('');
		if (reply.truncated) return out;
	}

	if (reply.overloaded) return out + sseFrame({ error: { message: OVERLOADED } });

	return (
		out +
		sseFrame({
			choices: [{ index: 0, delta: {}, finish_reason: reply.call ? 'tool_calls' : 'stop' }]
		}) +
		'data: [DONE]\n\n'
	);
};

/**
 * Gemini names no call ids on the wire and matches results back by function
 * name, so the ids the app runs on are its own. That is why the case worth
 * running here is a conversation with more than one call.
 */
const renderGemini = (reply: ModelReply): SseStream => {
	if (reply.truncated) {
		throw new Error('Gemini sends tool arguments already parsed; it cannot cut them short.');
	}
	const parts = [
		...(reply.text ? [{ text: reply.text }] : []),
		...(reply.call ? [{ functionCall: { name: reply.call.name, args: reply.call.input } }] : [])
	];
	const out = sseFrame({ candidates: [{ content: { parts }, finishReason: 'STOP' }] });
	return reply.overloaded ? out + sseFrame({ error: { message: OVERLOADED } }) : out;
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
 * reply in that provider's own wire format. Once the replies run out the last
 * is repeated, since the app keeps asking until a reply makes no tool call.
 */
export async function modelWillReply(page: Page, provider: AiProvider, replies: ModelReply[]) {
	// Rendered up front, so a reply this provider cannot express fails here
	// rather than inside a route handler, where it would stall the request.
	const streams = replies.map(provider.render);
	let sent = 0;
	await page.route(provider.url, async (route) => {
		const body = streams[Math.min(sent, streams.length - 1)];
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
