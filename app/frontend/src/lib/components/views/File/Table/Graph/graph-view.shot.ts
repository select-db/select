import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing,
	type Page
} from '../../../../../../../tests/e2e/shots';
import { testId, editor } from '../../../../../../../tests/e2e/selectors';

/**
 * The graph view, for the page that describes it.
 *
 * Sixty-odd lines of prose about chart types, axes, series and pivoting, with
 * no chart anywhere. It is the one feature on the site where words cannot do
 * the job at all.
 *
 * The chart is drawn from a real result: weekly_revenue.sql over the sample
 * database, twelve weeks of it. `revenue` is a formatted string and cannot be
 * plotted, which is itself worth knowing — the columns offered are the ones
 * that hold numbers.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

const FRAMING: Framing = { name: 'graph', width: 1000, height: 620, density: 2 };

const QUERY_FILE = 'weekly_revenue.sql';

/**
 * Picks options in one of the chart panel's selects, by its section label.
 *
 * The Y axis takes more than one column and leaves its menu open between
 * choices, so the trigger is clicked once and every option after it, then the
 * menu is closed the way a person would close it.
 */
async function choose(page: Page, section: string, options: string[]) {
	await page
		.locator('section')
		.filter({ has: page.locator('p.section-label', { hasText: section }) })
		.first()
		.locator('.select-container')
		.first()
		.click();
	for (const option of options) {
		await page.getByText(option, { exact: true }).last().click();
	}
	await page.keyboard.press('Escape');
}

for (const theme of THEMES) {
	test.describe(`graph ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('a result plotted', async ({ page, signIn }, info) => {
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

			await testId(page, 'tree.node', QUERY_FILE).click();
			await expect(editor.surface(page)).toBeVisible();

			await editor.surface(page).click();
			await page.keyboard.press('ControlOrMeta+Enter');
			// A currency cell can only come from the results table, so it is the result
			// arriving rather than the SQL above it.
			await expect(page.getByText(/^\$[\d,]+\.\d\d$/).first()).toBeVisible({ timeout: 20_000 });

			// The view switch is a radiogroup of icons with no accessible name --
			// the label a person sees is a tooltip on hover — so it is addressed
			// through the hook rather than through a title that is not there.
			await testId(page, 'segmented.option', 'graph').click();

			// The panel guesses, and its guess here is pivot mode on `revenue` --
			// which is a formatted string, so each of the twelve values becomes its
			// own series and the chart is twelve bell curves with a legend of
			// dollar amounts. Configure it the way the page describes instead:
			// weeks along the bottom, and the two columns that hold numbers.
			await choose(page, 'Series', ['None']);
			await choose(page, 'X Axis', ['week']);
			await choose(page, 'Y Axis', ['orders']);

			// The chart having re-rendered around those columns, not just the
			// selects having been clicked. Asserted through the legend, which only
			// exists once a series is drawn: both names are words in the SQL
			// above, so anywhere else on the page they would be visible whether
			// or not anything was ever plotted.
			const series = page.locator('.legend-label');
			await expect(series).toHaveCount(1, { timeout: 10_000 });

			await shot(page, shotsDirFor(info.file), `graph.${theme}`, FRAMING);
		});
	});
}
