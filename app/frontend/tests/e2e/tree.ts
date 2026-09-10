import { expect, type Page } from './wails';
import { renameBox, testId, treeRow } from './selectors';

/**
 * Driving the workspace tree. `selectors.ts` says how to address a row; this
 * says what to do to one. Separate because these carry waits and assumptions,
 * and a spec that reimplements one gets a subtly different gesture under the
 * same name.
 */

/** The workspace root's menu, opened on the empty space below the last row. */
export async function openRootMenu(page: Page) {
	const panel = testId(page, 'tree.panel');
	const box = await panel.boundingBox();
	if (!box) throw new Error('file tree is not on screen');

	await panel.click({ button: 'right', position: { x: 20, y: box.height - 20 } });
}

/** The context menu of one row. */
export async function openRowMenu(page: Page, name: string) {
	await treeRow(page, name).click({ button: 'right' });
}

/**
 * Picks an entry from whichever menu is open.
 *
 * Exactly, because the tree's menus carry both "Delete" and "Delete selected":
 * a loose match on the first would sometimes hit the second and take rows with
 * it that nobody named.
 */
export async function choose(page: Page, name: string) {
	await page.getByRole('menuitem', { name, exact: true }).click();
}

/**
 * Types a whole new name into the rename box that a new file, a new folder and
 * "Rename..." all open, and commits it.
 *
 * Select-all first: the box opens with only part of the name selected, up to
 * the extension. What is passed here is always the whole new name.
 *
 * The box closes whether the name was taken or not, so this returns once the
 * app has answered, not once it has agreed.
 */
export async function renameTo(page: Page, name: string) {
	const box = renameBox(page);
	await expect(box).toBeFocused();

	await box.press('ControlOrMeta+a');
	await box.fill(name);
	await box.press('Enter');

	await expect(box).toBeHidden();
}

/** Leaves the rename box without renaming, which is how a default name is kept. */
export async function keepName(page: Page) {
	const box = renameBox(page);
	await expect(box).toBeFocused();
	await box.press('Escape');
	await expect(box).toBeHidden();
}
