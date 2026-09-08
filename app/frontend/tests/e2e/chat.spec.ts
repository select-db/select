import { expect, test, holdSession, type Page } from './wails';
import { stubChatProvider, textTurn } from './shots';
import { toolCall, toolCallsInState, tabs, treeNode } from './selectors';

/**
 * The agent's tool calls, run for real against the seeded warehouse.
 *
 * Only the model's side is scripted — there is no model in a run — so what is
 * under test is the app around it: every call the model makes has to end in a
 * result. A card still showing its spinner after the conversation has moved on
 * is a call the app forgot, and the model is told "Tool execution did not
 * complete." for the rest of the session.
 *
 * The turns here are shaped like Anthropic's real stream: a text block first,
 * tool arguments arriving in fragments, and the events that wrap them. Streams
 * do not always end tidily, so several of these cut one short.
 */

const DB = 'sample-warehouse';

/** wails' id for DbClient.Query, from the generated bindings. */
const QUERY_CALL = 2964708639;

const sse = (type: string, data: object) =>
	`event: ${type}\ndata: ${JSON.stringify({ type, ...data })}\n\n`;

/** A turn that thinks out loud and then calls execute_query, args in fragments. */
function toolTurn(statement: string, id: string, thought: string) {
	const args = JSON.stringify({ dbInstanceId: DB, statement });
	const fragments = args.match(/.{1,7}/g) ?? [args];
	return (
		sse('message_start', { message: { id: 'msg_1', role: 'assistant', content: [] } }) +
		sse('content_block_start', { index: 0, content_block: { type: 'text', text: '' } }) +
		sse('content_block_delta', { index: 0, delta: { type: 'text_delta', text: thought } }) +
		sse('content_block_stop', { index: 0 }) +
		sse('ping', {}) +
		sse('content_block_start', {
			index: 1,
			content_block: { type: 'tool_use', id, name: 'execute_query', input: {} }
		}) +
		fragments
			.map((f) =>
				sse('content_block_delta', {
					index: 1,
					delta: { type: 'input_json_delta', partial_json: f }
				})
			)
			.join('') +
		sse('content_block_stop', { index: 1 }) +
		sse('message_delta', { delta: { stop_reason: 'tool_use' }, usage: { output_tokens: 1 } }) +
		sse('message_stop', {})
	);
}

/** The same turn, cut off by the error Anthropic sends when it is overloaded. */
const overloadedAfter = (turn: string) =>
	turn.replace(sse('message_stop', {}), '') +
	sse('error', { error: { type: 'overloaded_error', message: 'Overloaded' } });

/** Holds every execute_query call open until released, to act while one runs. */
async function holdQueries(page: Page) {
	let release!: () => void;
	const gate = new Promise<void>((resolve) => (release = resolve));
	let started = 0;
	await page.route('**/wails/runtime', async (route) => {
		if (!(route.request().postData() ?? '').includes(`"methodID":${QUERY_CALL}`)) {
			await route.fallback();
			return;
		}
		started += 1;
		await gate;
		await route.continue();
	});
	return { release: () => release(), started: () => started };
}

async function openChat(page: Page, signIn: () => Promise<void>, message = 'What is in there?') {
	await page.goto('/');
	await signIn();
	await expect(treeNode(page, 'weekly_revenue.sql')).toBeVisible();
	await page.getByRole('button', { name: 'New Chat' }).click();
	const prompt = page.getByRole('textbox', { name: 'Type a message...' });
	await expect(prompt).toBeVisible();
	await prompt.click();
	await page.keyboard.type(message);
	await page.keyboard.press('Enter');
}

async function say(page: Page, message: string) {
	const prompt = page.getByRole('textbox', { name: 'Type a message...' });
	await prompt.click();
	await page.keyboard.type(message);
	await page.keyboard.press('Enter');
}

/** Every call on screen has finished, whatever it finished as. */
async function expectSettled(page: Page, count: number) {
	await expect(toolCall(page)).toHaveCount(count, { timeout: 20_000 });
	await expect(toolCallsInState(page, 'running')).toHaveCount(0, { timeout: 20_000 });
}

test('runs the queries a conversation asks for', async ({ page, signIn, consoleErrors }) => {
	await holdSession(page);
	await stubChatProvider(page, [
		toolTurn('SELECT COUNT(*) FROM orders', 'toolu_1', 'Let me count the orders.'),
		toolTurn('SELECT status FROM orders LIMIT 3', 'toolu_2', 'And their statuses.'),
		textTurn('Both came back.')
	]);
	await openChat(page, signIn);

	await expect(page.getByText('Both came back.')).toBeVisible({ timeout: 20_000 });
	await expectSettled(page, 2);
	expect(consoleErrors).toEqual([]);
});

test('a provider error mid-stream does not strand the call that turn made', async ({
	page,
	signIn
}) => {
	// Anthropic ends an overloaded stream with an error event, after the tool
	// call is already complete. The call is good; only the stream broke.
	await holdSession(page);
	await stubChatProvider(page, [
		overloadedAfter(toolTurn('SELECT COUNT(*) FROM orders', 'toolu_1', 'Counting.')),
		textTurn('Recovered.')
	]);
	await openChat(page, signIn);

	await expectSettled(page, 1);
});

test('a second overloaded turn does not strand either call', async ({ page, signIn }) => {
	await holdSession(page);
	await stubChatProvider(page, [
		overloadedAfter(
			toolTurn(
				'SELECT DISTINCT status FROM orders LIMIT 20',
				'toolu_1',
				'Let me query the statuses.'
			)
		),
		overloadedAfter(
			toolTurn('SELECT COUNT(*) FROM orders', 'toolu_2', 'Let me check another way.')
		),
		textTurn('Recovered.')
	]);
	await openChat(page, signIn, 'What status values exist?');

	await expect(toolCall(page)).toHaveCount(1, { timeout: 20_000 });
	await say(page, 'stuck ?');

	await expectSettled(page, 2);
});

test('a turn cut off mid-arguments settles as a failed call', async ({ page, signIn }) => {
	await holdSession(page);
	const truncated =
		sse('content_block_start', {
			index: 0,
			content_block: { type: 'tool_use', id: 'toolu_cut', name: 'execute_query' }
		}) +
		sse('content_block_delta', {
			index: 0,
			delta: { type: 'input_json_delta', partial_json: '{"dbInstanceId":"sample-ware' }
		});
	await stubChatProvider(page, [truncated, textTurn('Recovered.')]);
	await openChat(page, signIn);

	await expect(toolCallsInState(page, 'failed')).toHaveCount(1, { timeout: 20_000 });
	await expectSettled(page, 1);
});

test('a call to a tool the app does not have settles as a failed call', async ({
	page,
	signIn
}) => {
	await holdSession(page);
	const unknown =
		sse('content_block_start', {
			index: 0,
			content_block: { type: 'tool_use', id: 'toolu_unknown', name: 'run_migration' }
		}) +
		sse('content_block_delta', {
			index: 0,
			delta: { type: 'input_json_delta', partial_json: '{"name":"x"}' }
		}) +
		sse('message_delta', { delta: { stop_reason: 'tool_use' } });
	await stubChatProvider(page, [unknown, textTurn('No such tool.')]);
	await openChat(page, signIn);

	await expect(toolCallsInState(page, 'failed')).toHaveCount(1, { timeout: 20_000 });
	await expectSettled(page, 1);
});

test('a query that fails at the transport settles as a failed call', async ({ page, signIn }) => {
	await holdSession(page);
	await page.route('**/wails/runtime', async (route) => {
		if ((route.request().postData() ?? '').includes(`"methodID":${QUERY_CALL}`)) {
			await route.fulfill({ status: 500, body: 'boom' });
			return;
		}
		await route.fallback();
	});
	await stubChatProvider(page, [
		toolTurn('SELECT COUNT(*) FROM orders', 'toolu_1', 'Counting.'),
		textTurn('That failed.')
	]);
	await openChat(page, signIn);

	await expect(page.getByText('That failed.')).toBeVisible({ timeout: 20_000 });
	await expectSettled(page, 1);
});

test('a second chat tab does not strand the query the first one started', async ({
	page,
	signIn
}) => {
	await holdSession(page);
	const gate = await holdQueries(page);
	await stubChatProvider(page, [
		toolTurn('SELECT COUNT(*) FROM orders', 'toolu_1', 'Counting.'),
		textTurn('It came back.')
	]);
	await openChat(page, signIn);

	await expect(toolCall(page)).toHaveCount(1, { timeout: 20_000 });
	await expect.poll(gate.started).toBeGreaterThan(0);

	// A second chat in the same group: the first panel is unmounted with its
	// query still running, and the result has nowhere to land unless the panel
	// that replaces it adopts the run.
	await page.getByRole('button', { name: 'Open Chat' }).click();
	await expect(tabs(page)).toHaveCount(2);
	gate.release();
	await tabs(page).first().click();

	await expect(page.getByText('It came back.')).toBeVisible({ timeout: 20_000 });
	await expectSettled(page, 1);
});

test('a restart with a query still running settles the restored call', async ({ page, signIn }) => {
	await holdSession(page);
	const gate = await holdQueries(page);
	await stubChatProvider(page, [
		toolTurn('SELECT COUNT(*) FROM orders', 'toolu_1', 'Counting.'),
		textTurn('It came back.')
	]);
	await openChat(page, signIn);

	await expect(toolCall(page)).toHaveCount(1, { timeout: 20_000 });
	await expect.poll(gate.started).toBeGreaterThan(0);

	gate.release();
	await page.reload();
	await signIn();

	await expectSettled(page, 1);
});
