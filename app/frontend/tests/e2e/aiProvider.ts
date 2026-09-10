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
	/** One turn, in this provider's wire format. */
	body: (turn: Turn) => string;
};

const OVERLOADED = 'Overloaded';

const frame = (payload: object, event?: string) =>
	`${event ? `event: ${event}\n` : ''}data: ${JSON.stringify(payload)}\n\n`;

/**
 * Providers stream tool arguments in fragments, never in one piece, and send
 * none at all for a call with no input: that is what leaves the app holding an
 * empty string rather than `{}`.
 */
const fragments = (input: object, truncated = false) => {
	const json = JSON.stringify(input);
	if (json === '{}') return [];
	const all = json.match(/.{1,7}/g) ?? [];
	return truncated ? all.slice(0, 1) : all;
};

const anthropic = (turn: Turn): string => {
	const sse = (type: string, data: object) => frame({ type, ...data }, type);
	let index = 0;
	let out = frame({
		type: 'message_start',
		message: { id: 'msg_1', role: 'assistant', content: [] }
	});

	if (turn.text) {
		out +=
			sse('content_block_start', { index, content_block: { type: 'text', text: '' } }) +
			sse('content_block_delta', { index, delta: { type: 'text_delta', text: turn.text } }) +
			sse('content_block_stop', { index });
		index += 1;
	}
	if (turn.call) {
		const args = fragments(turn.call.input, turn.truncated);
		out +=
			sse('ping', {}) +
			sse('content_block_start', {
				index,
				content_block: { type: 'tool_use', id: turn.call.id, name: turn.call.name, input: {} }
			}) +
			args
				.map((f) =>
					sse('content_block_delta', {
						index,
						delta: { type: 'input_json_delta', partial_json: f }
					})
				)
				.join('');
		if (turn.truncated) return out;
		out += sse('content_block_stop', { index });
	}

	if (turn.overloaded)
		return out + sse('error', { error: { type: 'overloaded_error', message: OVERLOADED } });

	return (
		out +
		sse('message_delta', { delta: { stop_reason: turn.call ? 'tool_use' : 'end_turn' } }) +
		sse('message_stop', {})
	);
};

/** The Chat Completions wire format, shared by OpenAI, xAI Grok and OpenRouter. */
const openAiCompatible = (turn: Turn): string => {
	const delta = (d: object) => frame({ choices: [{ index: 0, delta: d }] });
	let out = '';

	if (turn.text) out += delta({ content: turn.text });
	if (turn.call) {
		out += delta({
			tool_calls: [
				{
					index: 0,
					id: turn.call.id,
					type: 'function',
					function: { name: turn.call.name, arguments: '' }
				}
			]
		});
		out += fragments(turn.call.input, turn.truncated)
			.map((f) => delta({ tool_calls: [{ index: 0, function: { arguments: f } }] }))
			.join('');
		if (turn.truncated) return out;
	}

	if (turn.overloaded) return out + frame({ error: { message: OVERLOADED } });

	return (
		out +
		frame({
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
const gemini = (turn: Turn): string => {
	if (turn.truncated) {
		throw new Error('Gemini sends tool arguments already parsed; it cannot cut them short.');
	}
	const parts = [
		...(turn.text ? [{ text: turn.text }] : []),
		...(turn.call ? [{ functionCall: { name: turn.call.name, args: turn.call.input } }] : [])
	];
	const out = frame({ candidates: [{ content: { parts }, finishReason: 'STOP' }] });
	return turn.overloaded ? out + frame({ error: { message: OVERLOADED } }) : out;
};

/**
 * Every provider the app offers. The model is the first of each group in
 * aiConnections, which is the one a person is given by default.
 */
export const PROVIDERS: AiProvider[] = [
	{ model: 'claude-sonnet-5', url: 'https://api.anthropic.com/**', body: anthropic },
	{ model: 'gpt-5.6-sol', url: 'https://api.openai.com/**', body: openAiCompatible },
	{ model: 'gemini-3.7-flash', url: 'https://generativelanguage.googleapis.com/**', body: gemini },
	{ model: 'anthropic/claude-sonnet-5', url: 'https://openrouter.ai/**', body: openAiCompatible },
	{ model: 'grok-4.6', url: 'https://api.x.ai/**', body: openAiCompatible }
];

/**
 * The provider a fresh chat starts on: the first option whose provider has a
 * key, and so the one a spec gets without asking for it.
 */
export const ANTHROPIC = PROVIDERS[0];

/** Answers `provider` with `turns`, in order, repeating the last one. */
export async function stubProvider(page: Page, provider: AiProvider, turns: Turn[]) {
	// Rendered up front, so a turn this provider cannot express fails here rather
	// than inside a route handler, where it would surface as a stalled request.
	const bodies = turns.map(provider.body);
	let turn = 0;
	await page.route(provider.url, async (route) => {
		const body = bodies[Math.min(turn, bodies.length - 1)];
		turn += 1;
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
