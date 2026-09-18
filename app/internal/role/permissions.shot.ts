import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing
} from '../../frontend/tests/e2e/shots';

/**
 * The permission results table, for the Permissions page.
 *
 * That page is the one a team reads before paying for anything, and it
 * described five levels, five actions and a resolution order in prose alone.
 * The marketing figure beside "Stop asking someone for prod" shows the outcome
 * — a column somebody cannot read — and not the thing that produced it.
 *
 * This is the results table one level up, at the tables of a schema, where the actions
 * are the columns and a rule is a cell: the shape the page is actually about.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

/*
 * Wider than the other captures. The results table's columns are the actions, and the
 * page beside it counts five of them: a framing that cuts DELETE and DDL off
 * the right edge illustrates a claim it is also contradicting.
 */
const FRAMING: Framing = { name: 'permissions', width: 1180, height: 620, density: 2 };

const ROLE = 'analyst-readonly';

for (const theme of THEMES) {
	test.describe(`permissions ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('the grid a rule is written in', async ({ page, signIn }, info) => {
			await holdSession(page);
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

			await page.getByRole('button', { name: 'Open Settings' }).click();
			await page.getByRole('button', { name: 'Roles', exact: true }).click();

			await expect(page.getByText(ROLE).first()).toBeVisible({ timeout: 15_000 });
			await page.getByText(ROLE).first().click();

			// Down to the schema, so the rows are its tables. One level shallower
			// than the marketing figure: there a grant is one column wide, here it
			// is a results table of actions against objects, which is what the page reads
			// as a table of levels and actions.
			for (const level of ['warehouse', 'main']) {
				await page.getByRole('button', { name: level, exact: true }).first().click();
			}
			await expect(page.getByText('customers', { exact: true }).first()).toBeVisible({
				timeout: 15_000
			});

			await shot(page, shotsDirFor(info.file), `permissions.${theme}`, FRAMING);
		});
	});
}
