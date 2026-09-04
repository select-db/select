import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	stubChatProvider,
	textTurn,
	toolCallTurn,
	THEMES,
	expect,
	test,
	type Framing
} from '../../app/frontend/tests/e2e/shots';
import { testId } from '../../app/frontend/tests/e2e/selectors';

/**
 * The picture beside "Give your agent a key, not the keys": the chat asking for
 * a column the role does not grant, and being refused.
 *
 * The refusal is real. The seeded user holds analyst-readonly, which denies
 * select on customers.email, and the query tool returns that denial rather than
 * rows -- see seedRoles in internal/cmd/e2eseed. Only the model's side of the
 * conversation is scripted, because there is no model in a test run, and what
 * it says is checked against what the tools actually returned.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

/**
 * Narrow and tall, because that is the shape of a chat panel. The figure is
 * shown at 550px, and this is captured at 520 CSS wide: near enough to display
 * size that the text stays readable.
 */
const FRAMING: Framing = { name: 'agent', width: 1840, height: 820, density: 2 };

/**
 * Read the schema, then try to read the column, then explain the refusal. The
 * second call is the picture: the role denies select on customers.email, and
 * the tool says so in the words the app produced.
 */
const TURNS = [
	toolCallTurn('get_database_schemas', { databaseInstanceId: 'sample-warehouse' }, 'toolu_e2e_1'),
	toolCallTurn(
		'execute_query',
		{
			dbInstanceId: 'sample-warehouse',
			statement: 'SELECT email FROM customers LIMIT 5'
		},
		'toolu_e2e_2'
	),
	textTurn(
		'I can see the column, but I cannot read it: `analyst-readonly` denies ' +
			'`select` on `customers.email`, and the query came back refused rather ' +
			'than empty.\n\n' +
			'Anything that does not touch it still works. I can group the same orders ' +
			'by `country_code`, or return customer ids and let you join the addresses ' +
			'on your side.'
	)
];

for (const theme of THEMES) {
	test.describe(`agent ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('an agent refused the column its role does not grant', async ({ page, signIn }, info) => {
			await holdSession(page);
			await stubChatProvider(page, TURNS);
			await page.goto('/');
			await signIn();

			await page.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme);
			await expect
				.poll(() =>
					page.evaluate(() => {
						const [r, g, b] = (
							getComputedStyle(document.body).backgroundColor.match(/\d+/g) ?? ['255', '255', '255']
						).map(Number);
						return (r + g + b) / 3 < 128 ? 'dark' : 'light';
					})
				)
				.toBe(theme);

			// "New Chat" rather than the tab bar's chat action: that bar only exists
			// once a file is open, and this picture has no file in it.
			await page.getByRole('button', { name: 'New Chat' }).click();
			const prompt = page.getByRole('textbox', { name: 'Type a message...' });
			await expect(prompt).toBeVisible();
			await prompt.click();
			await page.keyboard.type('Which customers order the most? Give me their emails.');
			await page.keyboard.press('Enter');

			await expect(page.getByText('Read database schemas').first()).toBeVisible({
				timeout: 20_000
			});
			await expect(page.getByText('cannot read it', { exact: false })).toBeVisible({
				timeout: 20_000
			});

			// Open the refused call. Collapsed it reads as one more successful step;
			// the refusal, in the app's own words, is the whole picture.
			await page.getByText('Execute query', { exact: true }).first().click();

			// And assert those words, so a run where the role stopped being enforced
			// fails here instead of publishing a screenshot of rows.
			await expect(
				page.getByText('permission denied: select on main.customers.email', { exact: false })
			).toBeVisible({ timeout: 10_000 });

			await shot(
				page,
				shotsDirFor(info.file),
				`agent.${theme}`,
				FRAMING,
				testId(page, 'chat.panel')
			);
		});
	});
}
