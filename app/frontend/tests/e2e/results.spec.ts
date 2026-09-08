import { expect, open, test } from './wails';
import { editor, queryResultTable, testId } from './selectors';

/**
 * The results table's cell editor.
 *
 * One test, for one thing that nothing else here would notice: the modal a cell
 * expands into is monaco, and monaco is no longer in the bundle by the time this
 * runs. It arrives when the modal opens or it does not arrive at all, and an
 * empty modal is not something the app would report -- it would simply be empty.
 */

const QUERY =
	"SELECT id, status, total_cents FROM orders WHERE status = 'paid' ORDER BY id LIMIT 12;";

/** Columns as the query selects them; `id` is the key, so `status` is the editable one. */
const STATUS = 1;

test('a cell expands into an editor', async ({ page, signIn }) => {
	await open(page, signIn);

	await page.keyboard.press('ControlOrMeta+n');
	await expect(editor.surface(page)).toBeVisible();
	await editor.surface(page).click();
	await page.keyboard.type(QUERY);

	// A scratch tab starts attached to nothing, and the run is refused before it
	// ever reaches the database.
	await page.keyboard.press('ControlOrMeta+Shift+d');
	const picker = page.getByPlaceholder('Search db...');
	await expect(picker).toBeVisible({ timeout: 10_000 });
	await page.getByText('warehouse', { exact: true }).last().click();
	await page.keyboard.press('Escape');

	await editor.surface(page).click();
	await page.keyboard.press('ControlOrMeta+Enter');

	// The row count the toolbar reports, which only a result can set.
	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/12/, { timeout: 20_000 });

	await queryResultTable.cell(page, 0, STATUS).click();
	await expect(queryResultTable.cellInput(page)).toBeVisible();
	await page.keyboard.press('Shift+Enter');

	await expect(page.locator('.monaco-editor').last()).toBeVisible({ timeout: 20_000 });
});
