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
} from '../../../../../tests/e2e/shots';
import { testId, diffView } from '../../../../../tests/e2e/selectors';

/**
 * The chat proposing an edit, for the AI Chat page.
 *
 * Only the model's side is stubbed; the tools, the diff and its buttons are the
 * app's own. The edit is never approved, so the workspace is left as found --
 * every shot drives the same one.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

// Sized to the exchange: one question, one read, one proposal.
const FRAMING: Framing = { name: 'chat', width: 1400, height: 620, density: 2 };

const FILE = 'cohorts.sql';
const URI = `selectdb://workspaces/e2e-workspace/${FILE}`;

// `email` is absent because this workspace's role denies select on it.
const PROPOSED = `-- Cohort report, first cut.
--
-- Windowed by signup date, one row per customer.
SELECT
  c.id,
  c.country_code,
  c.created_at
FROM
  customers c
WHERE
  c.created_at >= '2026-01-05'
ORDER BY
  c.created_at DESC;
`;

const TURNS = [
	toolCallTurn('read_file', { uri: URI }, 'toolu_e2e_1'),
	toolCallTurn('edit_file', { uri: URI, content: PROPOSED }, 'toolu_e2e_2'),
	textTurn('Replaced the star with the three columns the report reads.')
];

for (const theme of THEMES) {
	test.describe(`chat ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('an edit proposed as a diff, waiting on approval', async ({ page, signIn }, info) => {
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

			// The chat picks up the open file, and the diff needs a group to open in.
			await testId(page, 'tree.node', FILE).click();

			// The tab bar's action; the empty state's "New Chat" is gone once a file is open.
			await page.getByRole('button', { name: 'Open Chat' }).click();
			const prompt = page.getByRole('textbox', { name: 'Type a message...' });
			await expect(prompt).toBeVisible();
			await prompt.click();
			await page.keyboard.type('This report selects everything. Narrow it to the columns it reads.');
			await page.keyboard.press('Enter');

			// Allow/Deny exist only on a diff awaiting approval, and appear twice --
			// on the diff and in the chat that asked. The figure needs both.
			await expect(diffView(page).getByRole('button', { name: 'Allow' })).toBeVisible({
				timeout: 20_000
			});
			await expect(
				testId(page, 'chat.panel').getByRole('button', { name: 'Deny' })
			).toBeVisible();
			await expect(diffView(page).getByText('Modified', { exact: true })).toBeVisible();

			await shot(page, shotsDirFor(info.file), `chat.${theme}`, FRAMING);
		});
	});
}
