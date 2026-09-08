import { expect, test, holdSession, type Page } from './wails';
import { PROVIDERS, chooseModel, stubProvider, type AiProvider, type Turn } from './aiProvider';
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
 * The first half runs against every provider, because how a turn arrives is the
 * only thing that differs between them: Anthropic and the Chat Completions
 * providers name their calls, Gemini does not, and each reports a broken stream
 * its own way. The second half is about the app rather than the wire, and runs
 * once.
 */

const DB = 'sample-warehouse';

/** wails' id for DbClient.Query, from the generated bindings. */
const QUERY_CALL = 2964708639;

const queryTurn = (statement: string, id: string, text: string, overloaded = false): Turn => ({
	text,
	call: { name: 'execute_query', input: { dbInstanceId: DB, statement }, id },
	overloaded
});

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

async function openChat(page: Page, signIn: () => Promise<void>, model?: string) {
	await page.goto('/');
	await signIn();
	await expect(treeNode(page, 'weekly_revenue.sql')).toBeVisible();
	await page.getByRole('button', { name: 'New Chat' }).click();
	if (model) await chooseModel(page, model);
	await expect(page.getByRole('textbox', { name: 'Type a message...' })).toBeVisible();
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

for (const provider of PROVIDERS satisfies AiProvider[]) {
	test.describe(provider.model, () => {
		test('runs the queries a conversation asks for', async ({ page, signIn, consoleErrors }) => {
			await holdSession(page);
			await stubProvider(page, provider, [
				queryTurn('SELECT COUNT(*) FROM orders', 'call_1', 'Let me count the orders.'),
				queryTurn('SELECT status FROM orders LIMIT 3', 'call_2', 'And their statuses.'),
				{ text: 'Both came back.' }
			]);
			await openChat(page, signIn, provider.model);
			await say(page, 'What is in there?');

			await expect(page.getByText('Both came back.')).toBeVisible({ timeout: 20_000 });
			await expectSettled(page, 2);
			expect(consoleErrors).toEqual([]);
		});

		test('a broken stream does not strand the call that turn made', async ({ page, signIn }) => {
			// Every provider can stop mid-stream once the tool call is already
			// complete — Anthropic sends an error event when it is overloaded, the
			// others an error chunk. The call is good; only the stream broke.
			await holdSession(page);
			await stubProvider(page, provider, [
				queryTurn('SELECT COUNT(*) FROM orders', 'call_1', 'Counting.', true),
				{ text: 'Recovered.' }
			]);
			await openChat(page, signIn, provider.model);
			await say(page, 'How many orders?');

			await expectSettled(page, 1);
		});
	});
}

const [anthropic] = PROVIDERS;

test('a second broken turn does not strand either call', async ({ page, signIn }) => {
	await holdSession(page);
	await stubProvider(page, anthropic, [
		queryTurn('SELECT DISTINCT status FROM orders LIMIT 20', 'call_1', 'Let me query them.', true),
		queryTurn('SELECT COUNT(*) FROM orders', 'call_2', 'Let me try another way.', true),
		{ text: 'Recovered.' }
	]);
	await openChat(page, signIn);
	await say(page, 'What status values exist?');

	await expect(toolCall(page)).toHaveCount(1, { timeout: 20_000 });
	await say(page, 'stuck ?');

	await expectSettled(page, 2);
});

test('a turn cut off mid-arguments settles as a failed call', async ({ page, signIn }) => {
	await holdSession(page);
	await stubProvider(page, anthropic, [
		{ ...queryTurn('SELECT COUNT(*) FROM orders', 'call_cut', 'Counting.'), truncated: true },
		{ text: 'Recovered.' }
	]);
	await openChat(page, signIn);
	await say(page, 'How many orders?');

	await expect(toolCallsInState(page, 'failed')).toHaveCount(1, { timeout: 20_000 });
	await expectSettled(page, 1);
});

test('a call to a tool the app does not have settles as a failed call', async ({
	page,
	signIn
}) => {
	await holdSession(page);
	await stubProvider(page, anthropic, [
		{ call: { name: 'run_migration', input: { name: 'x' }, id: 'call_1' } },
		{ text: 'No such tool.' }
	]);
	await openChat(page, signIn);
	await say(page, 'Migrate it.');

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
	await stubProvider(page, anthropic, [
		queryTurn('SELECT COUNT(*) FROM orders', 'call_1', 'Counting.'),
		{ text: 'That failed.' }
	]);
	await openChat(page, signIn);
	await say(page, 'How many orders?');

	await expect(page.getByText('That failed.')).toBeVisible({ timeout: 20_000 });
	await expectSettled(page, 1);
});

test('a second chat tab does not strand the query the first one started', async ({
	page,
	signIn
}) => {
	await holdSession(page);
	const gate = await holdQueries(page);
	await stubProvider(page, anthropic, [
		queryTurn('SELECT COUNT(*) FROM orders', 'call_1', 'Counting.'),
		{ text: 'It came back.' }
	]);
	await openChat(page, signIn);
	await say(page, 'How many orders?');

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
	await stubProvider(page, anthropic, [
		queryTurn('SELECT COUNT(*) FROM orders', 'call_1', 'Counting.'),
		{ text: 'It came back.' }
	]);
	await openChat(page, signIn);
	await say(page, 'How many orders?');

	await expect(toolCall(page)).toHaveCount(1, { timeout: 20_000 });
	await expect.poll(gate.started).toBeGreaterThan(0);

	gate.release();
	await page.reload();
	await signIn();

	await expectSettled(page, 1);
});
