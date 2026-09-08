import { call, expect, open, test, type Page } from './wails';
import type { APIRequestContext } from '@playwright/test';
import { activeTab, editor, selectedTreeNodes, tab, tabs, testId, treeNode } from './selectors';

/**
 * Tabs: what the workbench does with them, not what they hold.
 *
 * A tab is a frame around something else — a file, a terminal, a settings page
 * — and each of those has its own spec. What is tested here is the frame: that
 * opening the same file twice does not open it twice, that closing the active
 * one leaves a sensible tab behind, that the group remembers where it has been,
 * that a tab can be dragged into a split and back out, and that a tab follows
 * the file underneath it when the file moves or goes away.
 *
 * Where a file's content is asserted it is only ever evidence of which tab is
 * on screen. Nothing here is about the editor.
 */

const FS = 'selectDb/internal/fs_provider.FSProvider';
const GRAPH = 'selectDb/internal/graph.Graph';

/** The seeded files, and the first line each one shows. */
const WEEKLY = '-- revenue by week, this quarter';
const COHORTS = '-- Cohort report, first cut.';

/** Opens a file from the tree and waits for its tab. */
async function openFromTree(page: Page, name: string) {
	await treeNode(page, name).click();
	await expect(tab(page, name)).toBeVisible();
}

/** The tree's own context menu, from the empty space below the last row. */
async function openTreeMenu(page: Page) {
	const panel = testId(page, 'tree.panel');
	const box = await panel.boundingBox();
	if (!box) throw new Error('file tree is not on screen');

	await panel.click({ button: 'right', position: { x: 20, y: box.height - 20 } });
}

/** Closes a tab through the cross it shows on hover. */
async function closeTab(page: Page, name: string) {
	await tab(page, name).hover();
	await page.getByRole('button', { name: `Close ${name}` }).click();
	await expect(tab(page, name)).toHaveCount(0);
}

/**
 * Runs a command in the workspace root, standing in for the things that change
 * a workspace without going through the app.
 */
async function inWorkspace(
	request: APIRequestContext,
	workspaceId: string,
	command: string,
	...args: string[]
) {
	const result = await call<{ exitCode: number; stderr: string }>(request, `${FS}.ExecuteCommand`, {
		workspaceId,
		command,
		args
	});
	if (result.exitCode !== 0) {
		throw new Error(`${command} ${args.join(' ')}: ${result.stderr}`);
	}
}

/** Reads a workspace file from disk, through the app's own provider. */
async function readWorkspaceFile(request: APIRequestContext, id: string, name: string) {
	const prefix = await call<string>(request, `${FS}.WorkspaceURIPrefix`);
	return call<string>(request, `${FS}.ReadFile`, { uri: `${prefix}${id}/${name}` });
}

async function workspaceId(request: APIRequestContext) {
	const workspace = await call<{ id: string }>(request, `${GRAPH}.GetWorkspaceGraph`);
	return workspace.id;
}

test('opens one tab per file, and closes them by every route there is', async ({
	page,
	signIn
}) => {
	await open(page, signIn);

	// Nothing open is a state of its own: no tab bar at all, and the workbench
	// offering the things there are to start.
	await expect(tabs(page)).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'New query' })).toBeVisible();

	// A file opens as the active tab.
	await openFromTree(page, 'weekly_revenue.sql');
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'weekly_revenue.sql');

	// A second file opens beside the first and takes over.
	await openFromTree(page, 'cohorts.sql');
	await expect(tabs(page)).toHaveCount(2);
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'cohorts.sql');

	// A tab opens next to the active one rather than at the end, so what is
	// opened from a file stays next to it.
	await tab(page, 'weekly_revenue.sql').click();
	await openFromTree(page, 'top_customers.sql');
	await expect(tabs(page)).toHaveText(['weekly_revenue.sql', 'top_customers.sql', 'cohorts.sql']);

	// Opening a file that is already open focuses its tab instead of opening a
	// second one on the same file.
	await openFromTree(page, 'cohorts.sql');
	await expect(tabs(page)).toHaveCount(3);
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'cohorts.sql');

	// A query that was never a file is a tab too — the plus at the end of the
	// row makes one — and there is nothing on disk for it: it is written when it
	// is saved somewhere, not before.
	await page.getByTitle('New SQL file').click();
	await expect(activeTab(page)).toHaveAttribute('data-test-value', '[temp].sql');
	await expect(treeNode(page, '[temp].sql')).toHaveCount(0);
	await closeTab(page, '[temp].sql');

	// Closing the active tab hands the group to its left-hand neighbour rather
	// than to whatever happens to be first.
	await tab(page, 'top_customers.sql').click();
	await closeTab(page, 'top_customers.sql');
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'weekly_revenue.sql');

	// The keyboard closes the active one too.
	await page.keyboard.press('ControlOrMeta+w');
	await expect(tab(page, 'weekly_revenue.sql')).toHaveCount(0);
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'cohorts.sql');

	// And the menu on a tab closes the rest of the group.
	await openFromTree(page, 'weekly_revenue.sql');
	await openFromTree(page, 'top_customers.sql');
	await tab(page, 'cohorts.sql').click({ button: 'right' });
	await page.getByRole('menuitem', { name: 'Close others', exact: true }).click();
	await expect(tabs(page)).toHaveText(['cohorts.sql']);

	// Closing the last tab puts the workbench back where it started.
	await tab(page, 'cohorts.sql').click({ button: 'right' });
	await page.getByRole('menuitem', { name: 'Close all', exact: true }).click();
	await expect(tabs(page)).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'New query' })).toBeVisible();
});

test('gives each tab back what it was showing', async ({ page, request, signIn }) => {
	await open(page, signIn);
	const id = await workspaceId(request);
	const run = (command: string, ...args: string[]) => inWorkspace(request, id, command, ...args);

	await openFromTree(page, 'weekly_revenue.sql');
	await expect(editor.line(page, WEEKLY)).toBeVisible();

	await openFromTree(page, 'cohorts.sql');
	await expect(editor.line(page, COHORTS)).toBeVisible();

	// Only the active tab is rendered, so switching is a remount: what comes
	// back has to be this tab's file and not the one before it.
	await tab(page, 'weekly_revenue.sql').click();
	await expect(editor.line(page, WEEKLY)).toBeVisible();
	await expect(editor.line(page, COHORTS)).toHaveCount(0);

	await tab(page, 'cohorts.sql').click();
	await expect(editor.line(page, COHORTS)).toBeVisible();
	await expect(editor.line(page, WEEKLY)).toHaveCount(0);

	// The tree follows the active tab: the row of the file being edited is the
	// one selected, so the panel points at what the workbench is showing.
	await expect(selectedTreeNodes(page)).toHaveCount(1);
	await expect(selectedTreeNodes(page)).toHaveAttribute('data-test-value', 'cohorts.sql');

	// A terminal does not take the tab you were in: it opens a half of its own
	// below, so what is being worked on stays on screen. Two halves, each with
	// its own active tab.
	await page.getByRole('button', { name: 'Open Terminal' }).click();
	await expect(tab(page, 'Terminal')).toBeVisible();
	await expect(testId(page, 'group.content')).toHaveCount(2);
	await expect(activeTab(page)).toHaveCount(2);
	await expect(editor.line(page, COHORTS)).toBeVisible();

	// A tab with no file behind it points at nothing, and says so by leaving the
	// tree with nothing selected rather than pointing at the file before it.
	await expect(selectedTreeNodes(page)).toHaveCount(0);

	await tab(page, 'weekly_revenue.sql').click();
	await expect(selectedTreeNodes(page)).toHaveAttribute('data-test-value', 'weekly_revenue.sql');
	await expect(editor.line(page, WEEKLY)).toBeVisible();

	// Closing the last tab of a half takes the half with it, rather than leaving
	// an empty one on screen.
	await closeTab(page, 'Terminal');
	await expect(testId(page, 'group.content')).toHaveCount(1);

	// A query with no file under it is held by the tab itself, so switching away
	// from one is where it would be lost: there is nothing on disk to read it
	// back from.
	await page.getByTitle('New SQL file').click();
	await editor.surface(page).click();
	await page.keyboard.type('SELECT 41 + 1;');
	await expect(editor.line(page, 'SELECT 41 + 1;')).toBeVisible();

	await tab(page, 'weekly_revenue.sql').click();
	await expect(editor.line(page, WEEKLY)).toBeVisible();
	await tab(page, '[temp].sql').click();
	await expect(editor.line(page, 'SELECT 41 + 1;')).toBeVisible();

	// A file is written on a debounce, so leaving its tab straight after typing
	// is the moment the last keystrokes would go missing. They are on disk.
	await tab(page, 'weekly_revenue.sql').click();
	await editor.surface(page).click();
	await page.keyboard.press('ControlOrMeta+End');
	await page.keyboard.type('\n-- checked');
	await tab(page, '[temp].sql').click();

	await expect
		.poll(() => readWorkspaceFile(request, id, 'weekly_revenue.sql'))
		.toContain('-- checked');

	await run('git', 'checkout', '--', 'weekly_revenue.sql');
});

test('walks back and forward through the tabs it has been in', async ({ page, signIn }) => {
	await open(page, signIn);

	const back = page.getByRole('button', { name: 'Previous tab' });
	const forward = page.getByRole('button', { name: 'Next tab' });

	await openFromTree(page, 'weekly_revenue.sql');

	// One tab visited is nowhere to go, in either direction.
	await expect(back).toBeDisabled();
	await expect(forward).toBeDisabled();

	await openFromTree(page, 'cohorts.sql');
	await openFromTree(page, 'top_customers.sql');

	// Back walks the order they were visited in, not the order they sit in.
	await back.click();
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'cohorts.sql');
	await back.click();
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'weekly_revenue.sql');
	await expect(back).toBeDisabled();

	await forward.click();
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'cohorts.sql');

	// A tab that is closed leaves the history with it: walking back cannot land
	// on something that is no longer open.
	await closeTab(page, 'cohorts.sql');
	await back.click();
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'weekly_revenue.sql');
	await expect(tabs(page)).toHaveCount(2);
});

test('reorders tabs, splits the workbench with one, and takes it back', async ({
	page,
	signIn
}) => {
	await open(page, signIn);

	await openFromTree(page, 'weekly_revenue.sql');
	await openFromTree(page, 'cohorts.sql');
	await expect(tabs(page)).toHaveText(['weekly_revenue.sql', 'cohorts.sql']);

	// Dropped on the left half of a tab, a tab lands before it.
	const first = tab(page, 'weekly_revenue.sql');
	const box = await first.boundingBox();
	if (!box) throw new Error('the tabs are not on screen');
	await tab(page, 'cohorts.sql').dragTo(first, { targetPosition: { x: 4, y: box.height / 2 } });
	await expect(tabs(page)).toHaveText(['cohorts.sql', 'weekly_revenue.sql']);

	// Dropped on the side of what a tab is showing, it splits the workbench and
	// goes to live in the new half.
	const content = testId(page, 'group.content').first();
	const area = await content.boundingBox();
	if (!area) throw new Error('the workbench is not on screen');
	await tab(page, 'cohorts.sql').dragTo(content, {
		targetPosition: { x: area.width - 20, y: area.height / 2 }
	});

	await expect(testId(page, 'group.content')).toHaveCount(2);
	await expect(activeTab(page)).toHaveCount(2);
	await expect(tabs(page)).toHaveText(['weekly_revenue.sql', 'cohorts.sql']);

	// The two halves are resized by the handle between them.
	const resizer = testId(page, 'split.resizer');
	const before = await testId(page, 'group.content').first().boundingBox();
	const handle = await resizer.boundingBox();
	if (!before || !handle) throw new Error('the split is not on screen');

	await page.mouse.move(handle.x + handle.width / 2, handle.y + handle.height / 2);
	await page.mouse.down();
	await page.mouse.move(handle.x - 120, handle.y + handle.height / 2, { steps: 8 });
	await page.mouse.up();

	const after = await testId(page, 'group.content').first().boundingBox();
	if (!after) throw new Error('the split is not on screen');
	expect(after.width).toBeLessThan(before.width - 50);

	// Dropped in the middle of the other half, it moves there and the split is
	// gone: a half with nothing in it is not a half.
	const remaining = testId(page, 'group.content').first();
	const target = await remaining.boundingBox();
	if (!target) throw new Error('the workbench is not on screen');
	await tab(page, 'cohorts.sql').dragTo(remaining, {
		targetPosition: { x: target.width / 2, y: target.height / 2 }
	});

	await expect(testId(page, 'group.content')).toHaveCount(1);
	await expect(tabs(page)).toHaveCount(2);
	await expect(testId(page, 'split.resizer')).toHaveCount(0);
});

test('follows the files it has open', async ({ page, request, signIn }) => {
	await open(page, signIn);
	const id = await workspaceId(request);
	const run = (command: string, ...args: string[]) => inWorkspace(request, id, command, ...args);

	await openFromTree(page, 'weekly_revenue.sql');
	await openFromTree(page, 'cohorts.sql');

	// A rename through the app moves the tab with the file rather than opening a
	// second tab on the new name.
	await treeNode(page, 'cohorts.sql').click({ button: 'right' });
	await page.getByRole('menuitem', { name: 'Rename...', exact: true }).click();
	const box = page.getByRole('textbox', { name: 'Name' });
	await box.press('ControlOrMeta+a');
	await box.fill('cohorts-2026.sql');
	await box.press('Enter');

	await expect(tab(page, 'cohorts-2026.sql')).toBeVisible();
	await expect(tab(page, 'cohorts.sql')).toHaveCount(0);
	await expect(tabs(page)).toHaveCount(2);
	await expect(editor.line(page, COHORTS)).toBeVisible();

	// A file taken away underneath the app closes its tab: it can no longer be
	// read, and the workbench must not be left showing a path that is gone.
	await run('rm', 'weekly_revenue.sql', 'weekly_revenue.sql.metadata.json');
	await expect(tab(page, 'weekly_revenue.sql')).toHaveCount(0);
	await expect(tabs(page)).toHaveCount(1);

	// So does a file inside a folder that is deleted whole.
	await run('mkdir', 'box');
	await run('cp', 'cohorts-2026.sql', 'box/inside.sql');
	await expect(treeNode(page, 'box')).toBeVisible();
	await treeNode(page, 'box').click();
	await openFromTree(page, 'inside.sql');

	await run('rm', '-rf', 'box');
	await expect(tab(page, 'inside.sql')).toHaveCount(0);

	// A database is not a file, and its tab goes the same way: the graph loses
	// the database, the tab showing its connection goes with it.
	await openTreeMenu(page);
	await page.getByRole('menuitem', { name: 'New Database...', exact: true }).click();
	await expect(treeNode(page, 'db #1')).toBeVisible();

	await treeNode(page, 'db #1').click({ button: 'right' });
	await page.getByRole('menuitem', { name: 'Edit...', exact: true }).click();
	await expect(tab(page, 'db #1')).toBeVisible();

	await treeNode(page, 'db #1').click({ button: 'right' });
	await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
	await expect(treeNode(page, 'db #1')).toHaveCount(0);
	await expect(tab(page, 'db #1')).toHaveCount(0);

	// Leaving the workspace as it was found — by name, not with a checkout of
	// everything: the seed leaves an edit uncommitted on purpose, and the git
	// view and the screenshots are of a workspace that has it.
	await run('git', 'checkout', '--', 'weekly_revenue.sql', 'weekly_revenue.sql.metadata.json');
	await run('mv', 'cohorts-2026.sql', 'cohorts.sql');
	await run('mv', 'cohorts-2026.sql.metadata.json', 'cohorts.sql.metadata.json');

	for (const seeded of ['weekly_revenue.sql', 'top_customers.sql', 'cohorts.sql', 'warehouse']) {
		await expect(treeNode(page, seeded)).toBeVisible();
	}
});
