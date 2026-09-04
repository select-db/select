import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing
} from '../../app/frontend/tests/e2e/shots';
import { testId, editor } from '../../app/frontend/tests/e2e/selectors';

/**
 * The three pictures in the Getting Started guide, which is
 * `web/content/introduction.doc.md` rather than a marketing page. They are
 * captured here so every product screenshot the site publishes comes out of one
 * `shots/` directory and is served from one path, whichever page shows it.
 *
 * All three are the seeded workspace as it already stands: the `warehouse`
 * database it ships with, and one of its `.sql` files. Nothing here creates a
 * database or a file, so what a reader sees is the same state every other shot
 * is taken from.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

/**
 * Sized for the docs column, which is 54rem wide less its padding — about
 * 780px. Captured a little above that and displayed at or below it, the same
 * bargain the feature-row figures make.
 */
const EDITOR: Framing = { name: 'getting-started', width: 940, height: 560, density: 2 };
/** The connection form is a short panel; at the editor's height most of the
 *  picture is the empty space under it. */
const FORM: Framing = { ...EDITOR, height: 380 };

/** What the seed ships: a database, and a query file to point at it. */
const DATABASE = 'warehouse';
/*
 * weekly_revenue.sql, not top_customers.sql. The seeded user holds the
 * analyst-readonly role, which denies select on customers.email, and
 * top_customers selects it -- so running that file produces a permission
 * error. Correct behaviour, and the exact opposite of what step 4 claims. This
 * file reads orders only, which the role allows.
 */
const QUERY_FILE = 'weekly_revenue.sql';
/** The DSN db.config.json holds — a variable, not a secret. */
const DSN = '$WAREHOUSE_DSN';

for (const theme of THEMES) {
	test.describe(`getting started ${theme}`, () => {
		test.use({
			viewport: { width: EDITOR.width, height: EDITOR.height },
			deviceScaleFactor: EDITOR.density ?? 1.5
		});

		test('the connection, the picker and a result', async ({ page, signIn }, info) => {
			const dir = shotsDirFor(info.file);

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

			// 1. The connection, as the guide's step 3 describes it. Opening the
			// database the seed already created shows the same form a new one is
			// filled in with.
			// Right-click and take "Edit..." from the menu. That is the same action
			// a double-click runs, chosen because it is the one that always runs:
			// a double-click on a tree row has to beat a deferred single-click
			// timer, and the row re-renders when that timer expands the node, so
			// the pair can arrive as two clicks and no dblclick at all. The form
			// that opens for a database that exists is the form a new one is
			// created with, so nothing here walks the New Database flow.
			await testId(page, 'tree.node', DATABASE).click({ button: 'right' });
			await page.getByText('Edit...', { exact: true }).click();
			await expect(testId(page, 'database.form')).toBeVisible();
			// The DSN is the point of the figure: it is a $VAR, which is what the
			// tip under this step tells the reader to do. Assert it rather than
			// trusting the form rendered something.
			await expect(testId(page, 'database.dsn').locator('input')).toHaveValue(DSN, {
				timeout: 15_000
			});
			// Resized for this one shot, then put back: the form is the only short
			// picture of the three.
			await page.setViewportSize({ width: FORM.width, height: FORM.height });
			await shot(page, dir, `getting-started.database.${theme}`, FORM);
			await page.setViewportSize({ width: EDITOR.width, height: EDITOR.height });

			// 2. The database picker, step 4's least guessable moment. Cmd+Shift+D
			// is `editor.toggleDatabasePicker` in the shipped keybindings, so this
			// is the shortcut the guide names, not a click that stands in for it.
			await testId(page, 'tree.node', QUERY_FILE).click();
			await expect(editor.surface(page)).toBeVisible();
			await page.keyboard.press('ControlOrMeta+Shift+d');
			// The menu's own search box, not the database's name: the picker's
			// button carries that name whether the menu is open or shut.
			const pickerMenu = page.getByPlaceholder('Search db...');
			await expect(pickerMenu).toBeVisible({ timeout: 10_000 });
			await shot(page, dir, `getting-started.picker.${theme}`, EDITOR);

			// 3. The result, which is what step 4 ends on. Close the picker first:
			// an open menu over the results table is a different picture.
			await page.keyboard.press('Escape');
			await expect(pickerMenu).toBeHidden();

			await editor.surface(page).click();
			await page.keyboard.press('ControlOrMeta+Enter');

			// The results table header arrives before the rows do, and an empty results table of
			// placeholder dashes is not a first query having run -- so wait for a
			// cell, and one whose shape belongs to the result rather than to the
			// SQL above it. A column name would be visible in both.
			await expect(page.getByText(/^\$[\d,]+\.\d\d$/).first()).toBeVisible({
				timeout: 20_000
			});

			// A refusal is a result too, and it renders where the results table does. This
			// figure claims a query ran, so say out loud that it did: without this
			// the shot silently publishes whatever the pane happens to hold.
			await expect(page.getByText(/permission denied/i)).toBeHidden();

			// The toolbar's millisecond is the only thing that differs between runs
			// of this capture, and it would rewrite the PNG every time for a number
			// nobody reads. Assert the app measured and rendered a real duration,
			// then pin the digits. The pinned value is one this query produces.
			const duration = testId(page, 'result.duration');
			await expect(duration).toHaveText(/^\d\d:\d\d\.\d\d\d$/);
			await duration.evaluate((el) => {
				el.textContent = '00:00.002';
			});
			await shot(page, dir, `getting-started.result.${theme}`, EDITOR);
		});
	});
}
