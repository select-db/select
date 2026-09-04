import type { Page } from '@playwright/test';

/**
 * Every selector that is not a role or a user-visible string lives here.
 * See README.md for the convention.
 */

/** `data-test="<area>.<element>"`, optionally narrowed by `data-test-value`. */
export const testId = (page: Page, name: string, value?: string) =>
	page.locator(
		value === undefined
			? `[data-test="${name}"]`
			: `[data-test="${name}"][data-test-value="${value}"]`
	);

/** One open tab, named by the label it shows. */
export const tab = (page: Page, label: string) => testId(page, 'tabs.tab', label);

/**
 * Monaco's own DOM, quarantined.
 *
 * These class names are the editor's internals, not ours: they are not covered
 * by the convention and they can change when monaco-editor is upgraded. Keeping
 * them in one file means an upgrade is one place to fix rather than a search
 * across every spec. Nothing outside this file should mention `.view-line`,
 * `.suggest-widget` or `.monaco-*`.
 */
export const editor = {
	surface: (page: Page) => testId(page, 'editor.surface'),
	lines: (page: Page) => testId(page, 'editor.surface').locator('.view-lines'),
	line: (page: Page, containing?: string) =>
		containing === undefined
			? testId(page, 'editor.surface').locator('.view-line')
			: testId(page, 'editor.surface').locator('.view-line', { hasText: containing }),
	/** A lint underline, which monaco draws as a decoration over the text. */
	warning: (page: Page) => testId(page, 'editor.surface').locator('.squiggly-warning').first(),
	/** One row of the completion popup. Rendered in an overlay, not in the surface. */
	completionItem: (page: Page, label: string) =>
		page.locator('.suggest-widget .monaco-list-row', { hasText: label }),
	/**
	 * The spans monaco paints text into. Each carries an `mtkN` class naming a
	 * colour from the theme, and `mtk1` is the default one every token wears
	 * until the tokenizer has run.
	 */
	tokens: (page: Page) => page.locator('.view-line span span')
};

/**
 * ResultsTable's own DOM, quarantined for the same reason monaco's is. These
 * classes come from ResultsTable.svelte only; the settings tables are the
 * separate system/Table component and none of this matches them.
 *
 * A cell has nothing user-visible to name it by: it is addressed by where it
 * sits, through the row and column indices the table itself puts on the span.
 */
export const queryResultTable = {
	/** The element that scrolls when the table is wider than its pane. */
	scroller: (page: Page) => page.locator('div.table.scrollable'),
	cell: (page: Page, row: number, column: number) =>
		page.locator(`span.text-cell[data-row="${row}"][data-col="${column}"]`),
	/** A cell holding an uncommitted edit, which the table marks green. */
	editedCell: (page: Page, row: number, column: number) =>
		page.locator(`span.text-cell.edited[data-row="${row}"][data-col="${column}"]`),
	/** The input a cell turns into while it is being typed in. */
	cellInput: (page: Page) => page.locator('input.editable-input')
};

/**
 * The diff view's own DOM, for telling it apart from everything else on screen.
 *
 * An agent's edit puts Allow and Deny in two places at once -- here and in the
 * chat panel that asked for them -- and a spec usually means one of them.
 */
export const diffView = (page: Page) => page.locator('.diff-view');

/**
 * The input under a label, found by the label's own text. The app's
 * standalone-input component carries no accessible name, so the label element
 * beside it is the only stable handle.
 */
export const labelledInput = (page: Page, label: string) =>
	page
		.locator('.standalone-input')
		.filter({ has: page.locator('p.label', { hasText: label }) })
		.first()
		.locator('input')
		.first();
