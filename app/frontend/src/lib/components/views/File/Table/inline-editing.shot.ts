import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing
} from '../../../../../../tests/e2e/shots';
import { testId, editor, queryResultTable } from '../../../../../../tests/e2e/selectors';

/**
 * Editing a cell, and the SQL that comes out of it, for the Inline Editing
 * page.
 *
 * The page's whole argument is that an edit does not reach the database: it
 * becomes an UPDATE in a file you read first. That is a claim about two
 * screens, and the page had neither.
 *
 * The query is typed into a scratch tab rather than taken from the workspace,
 * because editing needs the primary key in the result and none of the sample
 * queries select one: two are aggregates, and the third reads a column this
 * workspace's role is denied. A scratch tab also leaves nothing behind, which
 * matters when every capture drives the same workspace.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

// Wider than the other figures on purpose. The results table gives every column 280px,
// and at 1000 the fourth one does not fit: editing it scrolls the results table to keep
// its input in view, which slides `id` under the sticky row-number column and
// publishes what reads as an empty column.
const FRAMING: Framing = { name: 'inlineedit', width: 1160, height: 620, density: 2 };

/**
 * Rows with their primary key, which is what makes a cell editable. The status
 * filter is what makes the first edit deterministic: every row the results table shows
 * says `paid`, so changing one to `refunded` is a visible change whichever row
 * the database happens to put first.
 */
const QUERY = "SELECT id, status, total_cents FROM orders WHERE status = 'paid' ORDER BY id LIMIT 12;";

/** Columns in the order the query selects them; `id` is the key, so not editable. */
const STATUS = 1;
const TOTAL_CENTS = 2;

for (const theme of THEMES) {
	test.describe(`inline editing ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('an edited cell, and the UPDATE it produces', async ({ page, signIn }, info) => {
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

			await page.keyboard.press('ControlOrMeta+n');
			await expect(editor.surface(page)).toBeVisible();
			await editor.surface(page).click();
			await page.keyboard.type(QUERY);

			// Typed on one line, then laid out by the app's own formatter, which is
			// what cmd+s is bound to. Every other query in this workspace is stored
			// as that formatter's output, so a figure showing one long line is the
			// only place in the docs where SELECT's SQL does not look like SELECT's
			// SQL.
			await page.keyboard.press('ControlOrMeta+s');
			// A clause alone on its line is the formatting, and nothing else here
			// could put it there.
			await expect(editor.line(page, 'FROM')).toHaveText(/^\s*FROM\s*$/);

			// A scratch tab starts attached to nothing, so running it here only
			// produces "Select a database before running this file".
			await page.keyboard.press('ControlOrMeta+Shift+d');
			const picker = page.getByPlaceholder('Search db...');
			await expect(picker).toBeVisible({ timeout: 10_000 });
			await page.getByText('warehouse', { exact: true }).last().click();
			await page.keyboard.press('Escape');
			await expect(picker).toBeHidden();

			await editor.surface(page).click();
			await page.keyboard.press('ControlOrMeta+Enter');

			// The row count the toolbar reports, which only a result can set.
			// Every column name in this query is also a word in the SQL above it,
			// so waiting for one of those would pass over an empty results table.
			await expect(testId(page, 'segmented.option', 'results')).toHaveText(/12/, {
				timeout: 20_000
			});

			// Two rows rather than one, because the page's claim is that each
			// edited row becomes its own UPDATE, and because a text column and a
			// numeric one are quoted differently in the SQL that comes out.
			await editCell(0, STATUS, 'refunded');
			await editCell(2, TOTAL_CENTS, '0');

			// The header's own count of edited rows. It appears with the Review and
			// Rollback buttons, so it is also the assertion that the figure below
			// has something to show.
			await expect(page.getByText('2 edits', { exact: true })).toBeVisible();

			// Nothing scrolled sideways while the cells were being typed in, so the
			// primary key is still in the picture. It is why the query selects it
			// and why the cells are editable at all; a figure without it argues
			// against the page it illustrates.
			await expect.poll(() => queryResultTable.scroller(page).evaluate((el) => el.scrollLeft)).toBe(0);

			await shot(page, dir, `inlineedit.cell.${theme}`, FRAMING);

			await page.getByRole('button', { name: 'Review' }).click();

			// Review generates the statements and opens them as a temp file. Both
			// UPDATEs, so the shutter cannot catch a half-written buffer.
			await expect(editor.line(page, 'UPDATE')).toHaveCount(2, { timeout: 10_000 });

			await shot(page, dir, `inlineedit.review.${theme}`, FRAMING);

			/**
			 * Types a value into one cell and commits it, the way a person does:
			 * click the cell, replace what is there, press Enter.
			 */
			async function editCell(row: number, column: number, value: string) {
				await queryResultTable.cell(page, row, column).click();
				const input = queryResultTable.cellInput(page);
				await expect(input).toBeVisible();
				await input.press('ControlOrMeta+a');
				await page.keyboard.type(value);
				await page.keyboard.press('Enter');
				// The cell carries the edited class once the value is held, which is
				// what Review reads. Without it the next click can land while the
				// input is still mounted and go to the input instead of the results table.
				await expect(queryResultTable.editedCell(page, row, column)).toHaveText(value);
			}
		});
	});
}
