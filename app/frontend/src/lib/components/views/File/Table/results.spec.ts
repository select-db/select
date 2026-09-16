import type { Page } from '@playwright/test';

import { AFTER_QUERY, expect, open, test } from '../../../../../../tests/e2e/wails';
import { editor, queryResultTable, testId } from '../../../../../../tests/e2e/selectors';
import { MAX_AUTO_COLUMN_WIDTH, MIN_AUTO_COLUMN_WIDTH } from './helpers/columnManagement';

/**
 * The results table, for two things nothing else here would notice.
 *
 * The modal a cell expands into is monaco, which is no longer in the bundle by
 * the time this runs: it arrives when the modal opens or not at all, and an
 * empty modal is not something the app reports.
 *
 * Column widths come from measuring the first rows against the theme's own
 * font, which only a browser can do. A unit test would measure the fallback.
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
	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/12/, AFTER_QUERY);

	await queryResultTable.cell(page, 0, STATUS).click();
	await expect(queryResultTable.cellInput(page)).toBeVisible();
	await page.keyboard.press('Shift+Enter');

	await expect(page.locator('.monaco-editor').last()).toBeVisible(AFTER_QUERY);
});

// Single-digit ids against 400 characters of hex: the two ends of the clamp, in
// one result, so the assertion does not depend on the theme's exact metrics.
const WIDTHS_QUERY = 'SELECT id, hex(zeroblob(200)) AS wide FROM orders ORDER BY id LIMIT 5;';

test('columns are sized to the rows that arrived', async ({ page, signIn }) => {
	await open(page, signIn);
	await run(page, WIDTHS_QUERY);

	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/5/, AFTER_QUERY);

	// Non-optional: a null box means the header never laid out, which is a
	// different failure from one that laid out at the wrong width.
	const narrow = (await queryResultTable.header(page, 'id').boundingBox())!;
	const wide = (await queryResultTable.header(page, 'wide').boundingBox())!;

	expect(narrow.width).toBe(MIN_AUTO_COLUMN_WIDTH);
	expect(wide.width).toBe(MAX_AUTO_COLUMN_WIDTH);
});

/**
 * Six columns of 400 hex characters: every one of them is capped at
 * MAX_AUTO_COLUMN_WIDTH, so the table is several times wider than the pane and
 * scrolls in both directions.
 */
const WIDE_QUERY =
	'SELECT o1.id AS id, hex(zeroblob(200)) AS aaa, hex(zeroblob(200)) AS bbb, ' +
	'hex(zeroblob(200)) AS ccc, hex(zeroblob(200)) AS ddd, hex(zeroblob(200)) AS eee ' +
	'FROM orders o1, orders o2 LIMIT 400;';

/** Pins a column through the pin button its header shows on hover. */
async function pin(page: Page, column: string) {
	const header = queryResultTable.header(page, column);
	await header.hover();
	await header.getByRole('button').first().click();
}

/** Every rendered cell of one row, with what it is doing horizontally. */
async function rowGeometry(page: Page, row: number) {
	return page.evaluate((index) => {
		const cells = document.querySelectorAll(`div.table.scrollable tbody tr:nth-child(${index}) td`);
		return [...cells].map((cell) => ({
			sticky: cell.classList.contains('sticky'),
			left: Math.round(cell.getBoundingClientRect().left),
			width: Math.round(cell.getBoundingClientRect().width)
		}));
	}, row);
}

/**
 * A pinned column stays put, and it stays put for every row, however far the
 * table has been scrolled: a row rendered while scrolled is rendered with its
 * pinned cells like any other.
 */
test('a pinned column holds its place through a scroll', async ({ page, signIn }) => {
	await open(page, signIn);
	await run(page, WIDE_QUERY);
	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/400/, AFTER_QUERY);

	await pin(page, 'aaa');

	const pinned = queryResultTable.header(page, 'aaa');
	const before = (await pinned.boundingBox())!;

	// Right, then down: the rows below were not rendered when the column was
	// pinned, and the columns to the left have scrolled out from under it.
	await queryResultTable.scroller(page).evaluate((el) => {
		el.scrollLeft = 1400;
		el.scrollTop = 2000;
	});

	await expect
		.poll(async () => Math.round((await pinned.boundingBox())!.x))
		.toBe(Math.round(before.x));

	const cells = await rowGeometry(page, 2);
	expect(cells[1].sticky).toBe(true);
	expect(cells[1].left).toBe(Math.round(before.x));

	// The column after it is where the scroll left it, which is what says the
	// pinned one is holding still rather than the table having stopped moving.
	expect(cells[2].left).toBeLessThan(cells[1].left);
});

/**
 * Dragging a column's edge resizes the header and the rows under it together.
 * The rows are the point: they are what went on showing the old width until
 * something forced them to paint again.
 */
test('the rows follow the column being resized', async ({ page, signIn }) => {
	await open(page, signIn);
	await run(page, WIDE_QUERY);
	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/400/, AFTER_QUERY);

	// Pinned, so the resize moves a sticky offset as well as a column width.
	await pin(page, 'aaa');

	const header = queryResultTable.header(page, 'aaa');
	const box = (await header.boundingBox())!;
	const narrower = Math.round(box.width) - 200;

	await page.mouse.move(box.x + box.width - 2, box.y + box.height / 2);
	await page.mouse.down();
	await page.mouse.move(box.x + box.width - 2 - 200, box.y + box.height / 2, { steps: 10 });
	await page.mouse.up();

	// Off the table: hovering a row repaints it, which is what used to be
	// needed before the rows agreed with their header.
	await page.mouse.move(2, 2);

	await expect.poll(async () => Math.round((await header.boundingBox())!.width)).toBe(narrower);

	const headers = await page.evaluate(() =>
		[...document.querySelectorAll('div.table.scrollable thead th')].map((th) => ({
			left: Math.round(th.getBoundingClientRect().left),
			width: Math.round(th.getBoundingClientRect().width)
		}))
	);

	for (const row of [1, 2, 3]) {
		const cells = await rowGeometry(page, row);
		expect(cells).toHaveLength(headers.length);
		cells.forEach((cell, column) => {
			expect(
				{ left: cell.left, width: cell.width },
				`row ${row} column ${column} does not line up with its header`
			).toEqual({ left: headers[column].left, width: headers[column].width });
		});
	}
});
