import { AFTER_QUERY, QUERY_CALL, expect, open, intercept, test, type Page } from './wails';
import {
	ANTHROPIC,
	PROVIDERS,
	chooseModel,
	say,
	modelWillReply,
	type ModelReply
} from './aiProvider';
import { toolCall, toolCallsInState, tabs } from './selectors';

/**
 * The agent's tool calls, run for real against the seeded warehouse. Only the
 * model's side is scripted, so what is under test is the app around it: every
 * call the model makes has to end in a result. A card still spinning after the
 * conversation has moved on is a call the app forgot, and the model is told
 * "Tool execution did not complete." for the rest of the session.
 *
 * The first half runs against every provider, since how a reply arrives is all
 * that differs between them: Anthropic and the Chat Completions providers name
 * their calls, Gemini does not, and each reports a broken stream its own way.
 * The second half is about the app rather than the wire, and runs once.
 */

/** The id the sample workspace gives its one database (internal/sample). */
const WAREHOUSE = 'sample-warehouse';

/** A reply in which the model says something, then runs a query. */
const runs = (text: string, statement: string, callId: string): ModelReply => ({
	text,
	call: { name: 'execute_query', input: { dbInstanceId: WAREHOUSE, statement }, id: callId }
});

/** Holds every execute_query call open until released, to act while one runs. */
async function holdQueries(page: Page) {
	let release!: () => void;
	const gate = new Promise<void>((resolve) => (release = resolve));
	let started = 0;
	await intercept(page, QUERY_CALL, async (route) => {
		started += 1;
		await gate;
		await route.continue();
	});
	return { release, started: () => started };
}

async function openChat(page: Page, signIn: () => Promise<void>, model = ANTHROPIC.model) {
	await open(page, signIn);
	await page.getByRole('button', { name: 'New Chat' }).click();
	await chooseModel(page, model);
}

/** Every call on screen has finished, whatever it finished as. */
async function expectCallsFinished(page: Page, count: number) {
	await expect(toolCall(page)).toHaveCount(count, AFTER_QUERY);
	await expect(toolCallsInState(page, 'running')).toHaveCount(0, AFTER_QUERY);
}

for (const provider of PROVIDERS) {
	test.describe(provider.model, () => {
		test('runs the queries a conversation asks for', async ({ page, signIn, consoleErrors }) => {
			await modelWillReply(page, provider, [
				runs('Let me count the orders.', 'SELECT COUNT(*) FROM orders', 'call_1'),
				runs('And their statuses.', 'SELECT status FROM orders LIMIT 3', 'call_2'),
				{ text: 'Both came back.' }
			]);
			await openChat(page, signIn, provider.model);
			await say(page, 'What is in there?');

			await expect(page.getByText('Both came back.')).toBeVisible(AFTER_QUERY);
			await expectCallsFinished(page, 2);
			expect(consoleErrors).toEqual([]);
		});

		test('a broken stream does not strand the call that turn made', async ({ page, signIn }) => {
			// Anthropic sends an error event when it is overloaded, the others an
			// error chunk. Either way the call is complete and only the stream broke.
			await modelWillReply(page, provider, [
				{ ...runs('Counting.', 'SELECT COUNT(*) FROM orders', 'call_1'), overloaded: true },
				{ text: 'Recovered.' }
			]);
			await openChat(page, signIn, provider.model);
			await say(page, 'How many orders?');

			await expectCallsFinished(page, 1);
		});
	});
}

test('a second broken turn does not strand either call', async ({ page, signIn }) => {
	await modelWillReply(page, ANTHROPIC, [
		{
			...runs('Let me query them.', 'SELECT DISTINCT status FROM orders LIMIT 20', 'call_1'),
			overloaded: true
		},
		{
			...runs('Let me try another way.', 'SELECT COUNT(*) FROM orders', 'call_2'),
			overloaded: true
		},
		{ text: 'Recovered.' }
	]);
	await openChat(page, signIn);
	await say(page, 'What status values exist?');

	await expect(toolCall(page)).toHaveCount(1, AFTER_QUERY);
	await say(page, 'stuck ?');

	await expectCallsFinished(page, 2);
});

test('a turn cut off mid-arguments settles as a failed call', async ({ page, signIn }) => {
	await modelWillReply(page, ANTHROPIC, [
		{ ...runs('Counting.', 'SELECT COUNT(*) FROM orders', 'call_cut'), truncated: true },
		{ text: 'Recovered.' }
	]);
	await openChat(page, signIn);
	await say(page, 'How many orders?');

	await expect(toolCallsInState(page, 'failed')).toHaveCount(1, AFTER_QUERY);
	await expectCallsFinished(page, 1);
});

test('a call that carries no arguments at all still runs', async ({ page, signIn }) => {
	// A provider sends no argument deltas for a call with no input, which leaves
	// the empty string. That is `{}`, not a broken stream: the tool runs and says
	// what it thinks of being given nothing.
	await modelWillReply(page, ANTHROPIC, [
		{ call: { name: 'execute_query', input: {}, id: 'call_bare' } },
		{ text: 'It said no.' }
	]);
	await openChat(page, signIn);
	await say(page, 'Run a query.');

	await expect(page.getByText('It said no.')).toBeVisible(AFTER_QUERY);
	await expectCallsFinished(page, 1);
	// The tool's own complaint, not one the app invented on its behalf.
	await toolCall(page).click();
	await expect(page.getByText('failed to get DB instance', { exact: false })).toBeVisible();
});

test('a call to a tool the app does not have settles as a failed call', async ({
	page,
	signIn
}) => {
	await modelWillReply(page, ANTHROPIC, [
		{ call: { name: 'run_migration', input: { name: 'x' }, id: 'call_1' } },
		{ text: 'No such tool.' }
	]);
	await openChat(page, signIn);
	await say(page, 'Migrate it.');

	await expect(toolCallsInState(page, 'failed')).toHaveCount(1, AFTER_QUERY);
	await expectCallsFinished(page, 1);
});

test('a query that fails at the transport settles as a failed call', async ({ page, signIn }) => {
	await intercept(page, QUERY_CALL, (route) => route.fulfill({ status: 500, body: 'boom' }));
	await modelWillReply(page, ANTHROPIC, [
		runs('Counting.', 'SELECT COUNT(*) FROM orders', 'call_1'),
		{ text: 'That failed.' }
	]);
	await openChat(page, signIn);
	await say(page, 'How many orders?');

	await expect(page.getByText('That failed.')).toBeVisible(AFTER_QUERY);
	await expectCallsFinished(page, 1);
});

test('a second chat tab does not strand the query the first one started', async ({
	page,
	signIn
}) => {
	const gate = await holdQueries(page);
	await modelWillReply(page, ANTHROPIC, [
		runs('Counting.', 'SELECT COUNT(*) FROM orders', 'call_1'),
		{ text: 'It came back.' }
	]);
	await openChat(page, signIn);
	await say(page, 'How many orders?');

	await expect(toolCall(page)).toHaveCount(1, AFTER_QUERY);
	await expect.poll(gate.started).toBeGreaterThan(0);

	// A second chat in the same group: the first panel is unmounted with its
	// query still running, and the result has nowhere to land unless the panel
	// that replaces it adopts the run.
	await page.getByRole('button', { name: 'Open Chat' }).click();
	await expect(tabs(page)).toHaveCount(2);
	gate.release();
	await tabs(page).first().click();

	await expect(page.getByText('It came back.')).toBeVisible(AFTER_QUERY);
	await expectCallsFinished(page, 1);
});

test('a restart with a query still running settles the restored call', async ({ page, signIn }) => {
	const gate = await holdQueries(page);
	await modelWillReply(page, ANTHROPIC, [
		runs('Counting.', 'SELECT COUNT(*) FROM orders', 'call_1'),
		{ text: 'It came back.' }
	]);
	await openChat(page, signIn);
	await say(page, 'How many orders?');

	await expect(toolCall(page)).toHaveCount(1, AFTER_QUERY);
	await expect.poll(gate.started).toBeGreaterThan(0);

	gate.release();
	await page.reload();
	await signIn();

	await expectCallsFinished(page, 1);
});
