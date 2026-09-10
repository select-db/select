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

/** One row of a tree (file, folder or database), named by what it shows. */
export const treeRow = (page: Page, name: string) => testId(page, 'tree.node', name);

/**
 * The rename box, wherever in the tree it is open.
 *
 * Scoped to the tree on purpose: a database's form has a Name field of its own,
 * and a spec that renames from the tree while that form is open would otherwise
 * be holding two textboxes called Name.
 */
export const renameBox = (page: Page) =>
	testId(page, 'tree.panel').getByRole('textbox', { name: 'Name' });

/**
 * The connection dot a database wears, by the database's id.
 *
 * Its `data-test-state` is what the app currently believes: "online", "offline",
 * or "unknown" for one nothing has reached yet.
 */
export const dbStatus = (page: Page, id: string) => testId(page, 'db.status', id);

/** The tree rows currently selected, in whichever trees are on screen. */
export const selectedRows = (page: Page) =>
	page.locator('[data-test="tree.node"][data-test-selected="true"]');

/** One open tab, named by the label it shows. */
export const tab = (page: Page, name: string) => testId(page, 'tabs.tab', name);

/** The tabs on screen, in the order they are laid out. */
export const tabs = (page: Page) => testId(page, 'tabs.tab');

/** The active tab of each group, so more than one once the workbench is split. */
export const activeTab = (page: Page) =>
	page.locator('[data-test="tabs.tab"][data-test-active="true"]');

/** One tool call card in the chat, named by the tool it calls. */
export const toolCall = (page: Page, name?: string) => testId(page, 'chat.tool-call', name);

/**
 * The tool cards in one state: 'running', 'pending', 'ok' or 'failed'. Still
 * 'running' after the conversation moved on is a call that never finished.
 */
export const toolCallsInState = (page: Page, state: 'running' | 'pending' | 'ok' | 'failed') =>
	testId(page, 'chat.tool-state', state);

/**
 * Monaco's own DOM, quarantined: these class names are the editor's internals
 * and change when monaco-editor is upgraded, so an upgrade is one file to fix.
 * Nothing outside this file mentions `.view-line`, `.suggest-widget` or
 * `.monaco-*`.
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
	/** A column's header cell, named by the column the query selects. */
	header: (page: Page, column: string) =>
		page
			.locator('div.table.scrollable th')
			.filter({ has: page.getByText(column, { exact: true }) }),
	cell: (page: Page, row: number, column: number) =>
		page.locator(`span.text-cell[data-row="${row}"][data-col="${column}"]`),
	/** A cell holding an uncommitted edit, which the table marks green. */
	editedCell: (page: Page, row: number, column: number) =>
		page.locator(`span.text-cell.edited[data-row="${row}"][data-col="${column}"]`),
	/** The input a cell turns into while it is being typed in. */
	cellInput: (page: Page) => page.locator('input.editable-input')
};

/**
 * The diff view's own DOM. An agent's edit puts Allow and Deny in two places at
 * once, here and in the chat panel that asked for them, and a spec means one.
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
