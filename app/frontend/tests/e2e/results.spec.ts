import type { Page } from '@playwright/test';

import { expect, open, test } from './wails';
import { editor, queryResultTable, testId } from './selectors';
import {
	MAX_AUTO_COLUMN_WIDTH,
	MIN_AUTO_COLUMN_WIDTH
} from '../../src/lib/components/views/File/Table/helpers/columnManagement';

/**
 * The results table, for two things nothing else here would notice.
 *
 * The modal a cell expands into is monaco, and monaco is no longer in the bundle
 * by the time this runs. It arrives when the modal opens or it does not arrive at
 * all, and an empty modal is not something the app would report -- it would
 * simply be empty.
 *
 * Column widths come from measuring the first batch of rows against the theme's
 * own font, which only a browser can do. A unit test would be measuring the
 * fallback.
 */

const QUERY =
	"SELECT id, status, total_cents FROM orders WHERE status = 'paid' ORDER BY id LIMIT 12;";

/** Columns as the query selects them; `id` is the key, so `status` is the editable one. */
const STATUS = 1;

/** Types the query into a fresh scratch tab pointed at the sample database, and runs it. */
async function run(page: Page, query: string) {
	await page.keyboard.press('ControlOrMeta+n');
	await expect(editor.surface(page)).toBeVisible();
	await editor.surface(page).click();
	await page.keyboard.type(query);

	// A scratch tab starts attached to nothing, and the run is refused before it
	// ever reaches the database.
	await page.keyboard.press('ControlOrMeta+Shift+d');
	const picker = page.getByPlaceholder('Search db...');
	await expect(picker).toBeVisible({ timeout: 10_000 });
	await page.getByText('warehouse', { exact: true }).last().click();
	await page.keyboard.press('Escape');

	await editor.surface(page).click();
	await page.keyboard.press('ControlOrMeta+Enter');
}

test('a cell expands into an editor', async ({ page, signIn }) => {
	await open(page, signIn);
	await run(page, QUERY);

	// The row count the toolbar reports, which only a result can set.
	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/12/, { timeout: 20_000 });

	await queryResultTable.cell(page, 0, STATUS).click();
	await expect(queryResultTable.cellInput(page)).toBeVisible();
	await page.keyboard.press('Shift+Enter');

	await expect(page.locator('.monaco-editor').last()).toBeVisible({ timeout: 20_000 });
});

// Single-digit ids against 400 characters of hex: the two ends of the clamp, in
// one result, so the assertion does not depend on the theme's exact metrics.
const WIDTHS_QUERY = 'SELECT id, hex(zeroblob(200)) AS wide FROM orders ORDER BY id LIMIT 5;';

test('columns are sized to the rows that arrived', async ({ page, signIn }) => {
	await open(page, signIn);
	await run(page, WIDTHS_QUERY);

	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/5/, { timeout: 20_000 });

	const narrow = await queryResultTable.header(page, 'id').boundingBox();
	const wide = await queryResultTable.header(page, 'wide').boundingBox();

	expect(narrow?.width).toBe(MIN_AUTO_COLUMN_WIDTH);
	expect(wide?.width).toBe(MAX_AUTO_COLUMN_WIDTH);
});
