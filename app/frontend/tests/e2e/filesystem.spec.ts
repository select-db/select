import { expect, holdSession, test, type Page } from './wails';
import { editor, tab, testId, treeNode } from './selectors';

/**
 * File management: what a person does to the workspace tree in a session —
 * make a folder, put files in it, rename them, delete them.
 *
 * One scenario rather than a test per gesture. These steps are what the app
 * does in sequence, and each one is only meaningful on the state the last one
 * left: a rename is interesting because there is something beside it to
 * disturb, a delete because a tab is open on what is being deleted.
 *
 * Everything it makes, it removes, so the seeded workspace is unchanged at the
 * end — the same workspace the screenshot suite photographs and the other specs
 * read.
 */

/** The tree's own context menu, from the empty space below the last row. */
async function openTreeMenu(page: Page) {
	const panel = testId(page, 'tree.panel');
	const box = await panel.boundingBox();
	if (!box) throw new Error('file tree is not on screen');

	await panel.click({ button: 'right', position: { x: 20, y: box.height - 20 } });
}

/** The context menu of one row. */
async function openMenuOn(page: Page, name: string) {
	await treeNode(page, name).click({ button: 'right' });
}

/**
 * Types into the rename box a new file, a new folder and "Rename..." all open,
 * and commits it.
 *
 * The box opens with part of the name selected — up to the extension, so typing
 * keeps it — which is right for a person and ambiguous for a test, hence the
 * select-all first: what is typed here is the whole new name.
 */
async function renameTo(page: Page, name: string) {
	const box = page.getByRole('textbox', { name: 'Name' });
	await expect(box).toBeFocused();

	await box.press('ControlOrMeta+a');
	await box.fill(name);
	await box.press('Enter');

	await expect(box).toBeHidden();
}

test('creates, renames and deletes files and folders', async ({ page, signIn }) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();

	// What the workspace starts as.
	await expect(treeNode(page, 'weekly_revenue.sql')).toBeVisible();

	// A new folder is created named, and named again by the person making it.
	await openTreeMenu(page);
	await page.getByRole('menuitem', { name: 'New folder...' }).click();
	await renameTo(page, 'reports');
	await expect(treeNode(page, 'reports')).toBeVisible();

	// A file made inside it opens as a tab, and what is typed into it is
	// written to disk without a save.
	await openMenuOn(page, 'reports');
	await page.getByRole('menuitem', { name: 'New file...' }).click();
	await renameTo(page, 'daily.sql');
	await expect(treeNode(page, 'daily.sql')).toBeVisible();
	await expect(tab(page, 'daily.sql')).toBeVisible();

	await editor.surface(page).click();
	await page.keyboard.type('SELECT 1;');
	await expect(editor.line(page, 'SELECT 1;')).toBeVisible();

	// A second file, so the rename below has a neighbour to leave alone.
	await openMenuOn(page, 'reports');
	await page.getByRole('menuitem', { name: 'New file...' }).click();
	await renameTo(page, 'monthly.sql');
	await expect(treeNode(page, 'monthly.sql')).toBeVisible();

	// Renaming moves the file on disk. The row takes the new name, the tab
	// follows the file rather than closing on a path that no longer exists, and
	// the rest of the folder stays where it was.
	await openMenuOn(page, 'daily.sql');
	await page.getByRole('menuitem', { name: 'Rename...' }).click();
	await renameTo(page, 'weekly.sql');

	await expect(treeNode(page, 'weekly.sql')).toBeVisible();
	await expect(treeNode(page, 'daily.sql')).toHaveCount(0);
	await expect(treeNode(page, 'monthly.sql')).toBeVisible();

	// The renamed file is the one behind the second tab now, and it still holds
	// what was typed into it under its old name.
	await tab(page, 'weekly.sql').click();
	await expect(editor.line(page, 'SELECT 1;')).toBeVisible();

	// Deleting the open file closes its tab with it.
	await openMenuOn(page, 'weekly.sql');
	await page.getByRole('menuitem', { name: 'Delete' }).click();

	await expect(treeNode(page, 'weekly.sql')).toHaveCount(0);
	await expect(tab(page, 'weekly.sql')).toHaveCount(0);

	// Deleting the folder takes what is still inside it.
	await openMenuOn(page, 'reports');
	await page.getByRole('menuitem', { name: 'Delete' }).click();

	await expect(treeNode(page, 'reports')).toHaveCount(0);
	await expect(treeNode(page, 'monthly.sql')).toHaveCount(0);

	// And the workspace is as it was found.
	await expect(treeNode(page, 'weekly_revenue.sql')).toBeVisible();
});
